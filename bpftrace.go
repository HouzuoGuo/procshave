package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/procfs/blockdevice"
)

var (
	TcpAddrPortKeyRegex = regexp.MustCompile(`\[([0-9,-]+)\],([0-9]+)`)
)

type BpfMapRecord struct {
	Type string                    `json:"type"`
	Data map[string]map[string]int `json:"data"`
}

type BpfNetIOTrafficCounter struct {
	IP          net.IP
	Port        int
	ByteCounter int
	IsDest      bool
}

func TcpTrafficFromBpfMap(bpfMap map[string]int, isDest bool) []BpfNetIOTrafficCounter {
	/*
		Sample data for localhost communication:
		{"type": "map", "data": {"@tcp_src": {"[10,0,0,11,0,0,0,0,0,0,0,0,0,0,0,0,0,0,-1,-1,127,0,0,1,0,0,0,0],11": 0, "[2,0,-89,74,127,0,0,1,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0],42826": 73, "[2,0,-89,66,127,0,0,1,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0],42818": 73}}}
		{"type": "map", "data": {"@tcp_dest": {"[10,0,-89,66,0,0,0,0,0,0,0,0,0,0,0,0,0,0,-1,-1,127,0,0,1,0,0,0,0],42818": 0, "[10,0,-89,74,0,0,0,0,0,0,0,0,0,0,0,0,0,0,-1,-1,127,0,0,1,0,0,0,0],42826": 0, "[2,0,0,11,127,0,0,1,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0],11": 146}}}
	*/
	var ret []BpfNetIOTrafficCounter
	for addrPortKey, trafficBytes := range bpfMap {
		addrPort := TcpAddrPortKeyRegex.FindStringSubmatch(addrPortKey)
		if len(addrPort) != 3 {
			continue
		}
		sockAddrIn6Str := addrPort[1]
		port, _ := strconv.Atoi(addrPort[2])
		var sockAddrIn6 []byte
		for _, byteStr := range strings.Split(sockAddrIn6Str, ",") {
			byteVal, _ := strconv.Atoi(strings.TrimSpace(byteStr))
			sockAddrIn6 = append(sockAddrIn6, byte(byteVal))
		}
		if len(sockAddrIn6) != 28 {
			continue
		}
		var ipAddr net.IP
		switch sockAddrIn6[0] {
		case 2:
			ipAddr = net.IP(sockAddrIn6[4 : 4+4])
		case 10:
			ipAddr = net.IP(sockAddrIn6[4 : 4+16])
		default:
			continue
		}
		ret = append(ret, BpfNetIOTrafficCounter{
			IP:          ipAddr,
			Port:        port,
			ByteCounter: trafficBytes,
			IsDest:      isDest,
		})

	}
	sort.Slice(ret, func(i, j int) bool {
		return ret[i].ByteCounter > ret[j].ByteCounter
	})
	return ret
}

type BpfTracer struct {
	mutex               *sync.Mutex
	stop                chan struct{}
	PID                 int
	SamplingIntervalSec int
	Metrics             *MetricsCollector

	FDBytesRead   map[string]int
	FDBytesReadTS time.Time

	FDBytesWritten   map[string]int
	FDBytesWrittenTS time.Time

	TcpTrafficSources   []BpfNetIOTrafficCounter
	TcpTrafficSourcesTS time.Time

	TcpTrafficDestinations   []BpfNetIOTrafficCounter
	TcpTrafficDestinationsTS time.Time

	BlockDeviceIONanos   map[string]int
	BlockDeviceIONanosTS time.Time

	BlockDeviceIOSectors   map[string]int
	BlockDeviceIOSectorsTS time.Time
}

func NewBpfTracer(pid int, samplingIntervalSec int, metrics *MetricsCollector) *BpfTracer {
	return &BpfTracer{
		mutex:                new(sync.Mutex),
		stop:                 make(chan struct{}, 1),
		PID:                  pid,
		SamplingIntervalSec:  samplingIntervalSec,
		FDBytesRead:          make(map[string]int),
		FDBytesWritten:       make(map[string]int),
		BlockDeviceIONanos:   make(map[string]int),
		BlockDeviceIOSectors: make(map[string]int),
		Metrics:              metrics,
	}
}

func (bpf *BpfTracer) Start() error {
	code := fmt.Sprintf(`
tracepoint:syscalls:sys_enter_read /pid == %d/ {
	@fd[tid] = args->fd;
}
tracepoint:syscalls:sys_exit_read /pid == %d && @fd[tid]/ {
    if (args->ret > 0) {@read_fd[@fd[tid]] += args->ret;}
    delete(@fd[tid]);
}
tracepoint:syscalls:sys_enter_write /pid == %d/ {
    @fd[tid] = args->fd;
}
tracepoint:syscalls:sys_exit_write /pid == %d && @fd[tid]/ {
    if (args->ret > 0) {@write_fd[@fd[tid]] += args->ret;}
    delete(@fd[tid]);
}
tracepoint:tcp:tcp_probe /pid == %d/ {
    @tcp_src[args->saddr, args->sport] += args->data_len;
    @tcp_dest[args->daddr, args->dport] += args->data_len;
}
tracepoint:block:block_io_start /pid == %d/ {
    @blkdev_sector_count[args->dev] += args->nr_sector;
    @blkdev_req[args->sector] = nsecs;
}
tracepoint:block:block_io_done /@blkdev_req[args->sector] != 0/ {
    @blkdev_dur[args->dev] += nsecs - @blkdev_req[args->sector];
    delete(@blkdev_req[args->sector]);
}
interval:s:%d {
    print(@read_fd); print(@write_fd);
    print(@tcp_src); print(@tcp_dest);
    print(@blkdev_dur); print(@blkdev_sector_count);
    clear(@read_fd); clear(@write_fd);
    clear(@tcp_src); clear(@tcp_dest);
    clear(@blkdev_dur); clear(@blkdev_sector_count);
}
	`, bpf.PID, bpf.PID, bpf.PID, bpf.PID, bpf.PID, bpf.PID, bpf.SamplingIntervalSec)
	cmd := exec.Command("bpftrace", "-e", code, "-f", "json")
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}
	go func() {
		stderrReader := bufio.NewReader(stderr)
		for {
			line, err := stderrReader.ReadString('\n')
			log.Printf("bpftrace err: %v, stderr: %v", err, line)
			if err != nil {
				return
			}
		}
	}()
	if err := cmd.Start(); err != nil {
		return err
	}
	go bpf.Housekeeping()
	go func() {
		stdoutReader := bufio.NewReader(stdout)
		defer func() {
			close(bpf.stop)
		}()
		for {
			line, err := stdoutReader.ReadString('\n')
			if err != nil {
				return
			}
			log.Printf("bpftrace stdout: %s", line)
			bpf.unmarshalBpfRecord(line)
		}
	}()
	return cmd.Wait()
}

func (bpf *BpfTracer) unmarshalBpfRecord(line string) {
	var rec BpfMapRecord
	if err := json.Unmarshal([]byte(line), &rec); err != nil {
		return
	}
	if rec.Type == "map" && rec.Data != nil {
		if read := rec.Data["@read_fd"]; read != nil {
			bpf.mutex.Lock()
			bpf.FDBytesRead = read
			bpf.FDBytesReadTS = time.Now()
			bpf.mutex.Unlock()
		} else if written := rec.Data["@write_fd"]; written != nil {
			bpf.mutex.Lock()
			bpf.FDBytesWritten = written
			bpf.FDBytesWrittenTS = time.Now()
			bpf.mutex.Unlock()
		} else if tcpSrc := rec.Data["@tcp_src"]; tcpSrc != nil {
			bpf.mutex.Lock()
			bpf.TcpTrafficSources = TcpTrafficFromBpfMap(tcpSrc, false)
			bpf.TcpTrafficSourcesTS = time.Now()
			bpf.mutex.Unlock()
		} else if tcpDest := rec.Data["@tcp_dest"]; tcpDest != nil {
			bpf.mutex.Lock()
			bpf.TcpTrafficDestinations = TcpTrafficFromBpfMap(tcpDest, true)
			bpf.TcpTrafficDestinationsTS = time.Now()
			bpf.mutex.Unlock()
		} else if duration := rec.Data["@blkdev_dur"]; duration != nil {
			bpf.mutex.Lock()
			bpf.BlockDeviceIONanos = duration
			bpf.BlockDeviceIONanosTS = time.Now()
			bpf.mutex.Unlock()
		} else if sectors := rec.Data["@blkdev_sector_count"]; sectors != nil {
			bpf.mutex.Lock()
			bpf.BlockDeviceIOSectors = sectors
			bpf.BlockDeviceIOSectorsTS = time.Now()
			bpf.mutex.Unlock()
		}
	}
}

type FileIOCounter struct {
	Name                    string
	ReadBytes, WrittenBytes int
}

type FileIOSummary struct {
	ByName map[string]*FileIOCounter
	ByRate []*FileIOCounter
}

func (bpf *BpfTracer) FileIOSummary(fdPaths map[int]string) *FileIOSummary {
	ret := &FileIOSummary{
		ByName: make(map[string]*FileIOCounter),
		ByRate: []*FileIOCounter{},
	}
	for fd, read := range bpf.FDBytesRead {
		fdNum, _ := strconv.Atoi(fd)
		fileName, exists := fdPaths[fdNum]
		if !exists {
			continue
		}
		if _, exists := ret.ByName[fileName]; !exists {
			ret.ByName[fileName] = &FileIOCounter{Name: fileName}
		}
		ret.ByName[fileName].ReadBytes = read
	}
	for fd, written := range bpf.FDBytesWritten {
		fdNum, _ := strconv.Atoi(fd)
		fileName, exists := fdPaths[fdNum]
		if !exists {
			continue
		}
		if _, exists := ret.ByName[fileName]; !exists {
			ret.ByName[fileName] = &FileIOCounter{Name: fileName}
		}
		ret.ByName[fileName].WrittenBytes = written
	}

	for _, ioCounter := range ret.ByName {
		ret.ByRate = append(ret.ByRate, ioCounter)
	}
	sort.Slice(ret.ByRate, func(i, j int) bool {
		a := ret.ByRate[i]
		b := ret.ByRate[j]
		return a.ReadBytes+a.WrittenBytes > b.ReadBytes+b.WrittenBytes
	})
	return ret
}

type BlockIOCounter struct {
	DeviceName  string
	MajorMinor  string
	SectorCount int
	IODuration  time.Duration
}

type BlockIOSummary struct {
	ByName     map[string]*BlockIOCounter
	ByDuration []*BlockIOCounter
}

func devtMajorMinor(devt int) (int, int) {
	/*
	   dev_t example:
	   {"type": "map", "data": {"@blkdev_dur": {"8388608": 5888654}}}
	   {"type": "map", "data": {"@blkdev_sector_count": {"8388608": 11}}}
	*/
	return devt >> 20, (devt >> 8) & 0x7f
}

func (bpf *BpfTracer) BlockIOSummary(diskStats map[string]blockdevice.Diskstats) *BlockIOSummary {
	ret := &BlockIOSummary{
		ByName:     make(map[string]*BlockIOCounter),
		ByDuration: []*BlockIOCounter{},
	}
	for devt, duration := range bpf.BlockDeviceIONanos {
		devtNum, _ := strconv.Atoi(devt)
		major, minor := devtMajorMinor(devtNum)
		majorMinor := fmt.Sprintf("%d:%d", major, minor)
		disk, exists := diskStats[majorMinor]
		if !exists {
			continue
		}
		if _, exists := ret.ByName[disk.DeviceName]; !exists {
			ret.ByName[disk.DeviceName] = &BlockIOCounter{
				DeviceName: disk.DeviceName,
				MajorMinor: majorMinor,
				IODuration: time.Duration(duration) * time.Nanosecond,
			}
		}
	}
	for devt, sectors := range bpf.BlockDeviceIOSectors {
		devtNum, _ := strconv.Atoi(devt)
		major, minor := devtMajorMinor(devtNum)
		majorMinor := fmt.Sprintf("%d:%d", major, minor)
		disk, exists := diskStats[majorMinor]
		if !exists {
			continue
		}
		if ioCounter, exists := ret.ByName[disk.DeviceName]; exists {
			ioCounter.SectorCount = sectors
		}
	}
	for _, ioCounter := range ret.ByName {
		ret.ByDuration = append(ret.ByDuration, ioCounter)
	}
	sort.Slice(ret.ByDuration, func(i, j int) bool {
		a := ret.ByDuration[i]
		b := ret.ByDuration[j]
		return a.IODuration > b.IODuration
	})
	return ret
}

func (bpf *BpfTracer) Housekeeping() {
	ticker := time.Tick(time.Duration(bpf.SamplingIntervalSec) * time.Second)
	for {
		select {
		case <-ticker:
			bpf.mutex.Lock()
			if time.Since(bpf.FDBytesReadTS) > time.Duration(bpf.SamplingIntervalSec)*time.Second {
				bpf.FDBytesRead = make(map[string]int)
			}
			if time.Since(bpf.FDBytesWrittenTS) > time.Duration(bpf.SamplingIntervalSec)*time.Second {
				bpf.FDBytesWritten = make(map[string]int)
			}
			if time.Since(bpf.TcpTrafficSourcesTS) > time.Duration(bpf.SamplingIntervalSec)*time.Second {
				bpf.TcpTrafficSources = make([]BpfNetIOTrafficCounter, 0)
			}
			if time.Since(bpf.TcpTrafficDestinationsTS) > time.Duration(bpf.SamplingIntervalSec)*time.Second {
				bpf.TcpTrafficDestinations = make([]BpfNetIOTrafficCounter, 0)
			}
			if time.Since(bpf.BlockDeviceIONanosTS) > time.Duration(bpf.SamplingIntervalSec)*time.Second {
				bpf.BlockDeviceIONanos = make(map[string]int)
			}
			if time.Since(bpf.BlockDeviceIOSectorsTS) > time.Duration(bpf.SamplingIntervalSec)*time.Second {
				bpf.BlockDeviceIOSectors = make(map[string]int)
			}

			hostname, _ := os.Hostname()
			labels := prometheus.Labels{PidLabel: strconv.Itoa(bpf.PID), HostnameLabel: hostname}

			sum := 0
			for _, count := range bpf.FDBytesRead {
				sum += count
			}
			bpf.Metrics.ReadFromFDBytes.With(labels).Set(float64(sum) / float64(bpf.SamplingIntervalSec))
			bpf.Metrics.ReadFromFDCount.With(labels).Set(float64(len(bpf.FDBytesRead)) / float64(bpf.SamplingIntervalSec))

			sum = 0
			for _, count := range bpf.FDBytesWritten {
				sum += count
			}
			bpf.Metrics.WrittenToFDBytes.With(labels).Set(float64(sum) / float64(bpf.SamplingIntervalSec))
			bpf.Metrics.WrittenToFDCount.With(labels).Set(float64(len(bpf.FDBytesWritten)) / float64(bpf.SamplingIntervalSec))

			sum = 0
			for _, count := range bpf.TcpTrafficSources {
				sum += count.ByteCounter
			}
			bpf.Metrics.TcpSourceTrafficBytes.With(labels).Set(float64(sum) / float64(bpf.SamplingIntervalSec))
			bpf.Metrics.TcpSourceEndpointsCount.With(labels).Set(float64(len(bpf.TcpTrafficSources)) / float64(bpf.SamplingIntervalSec))

			sum = 0
			for _, count := range bpf.TcpTrafficDestinations {
				sum += count.ByteCounter
			}
			bpf.Metrics.TcpDestinationTrafficBytes.With(labels).Set(float64(sum) / float64(bpf.SamplingIntervalSec))
			bpf.Metrics.TcpDestinationEndpointsCount.With(labels).Set(float64(len(bpf.TcpTrafficDestinations)) / float64(bpf.SamplingIntervalSec))

			sum = 0
			for _, count := range bpf.BlockDeviceIOSectors {
				sum += count
			}
			bpf.Metrics.BlockIOSectors.With(labels).Set(float64(sum) / float64(bpf.SamplingIntervalSec))

			sum = 0
			for _, count := range bpf.BlockDeviceIONanos {
				sum += count
			}
			bpf.Metrics.BlockIOTimeMillis.With(labels).Set(float64(sum/1000000) / float64(bpf.SamplingIntervalSec))
			bpf.mutex.Unlock()
		case <-bpf.stop:
			return
		}
	}
}
