package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os/exec"
	"sort"
	"strconv"
	"strings"
)

type BpfFDRecord struct {
	Type string                    `json:"type"`
	Data map[string]map[string]int `json:"data"`
}

type BpfNetIORecord struct {
	Type string         `json:"type"`
	Data map[string]int `json:"data"`
}

type BpfNetIOTrafficCounter struct {
	IP          net.IP
	Port        int
	ByteCounter int
}

func (rec *BpfNetIORecord) GetEndpointTraffic() []BpfNetIOTrafficCounter {
	/*
	  Sampple data:
	  "[11,0,-72,3,0,0,0,0,0,0,0,0,0,0,0,0,0,0,-1,-1,127,0,0,1,0,0,0,0],36344": 156,
	  "[3,0,-80,-80,-64,-88,50,-60,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0],40792": 185,
	  "[5,0,9,-111,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,1,0,0,0,0],2144": 422,
	  "[5,0,-27,-50,0,0,0,0,0,0,0,0,0,0,0,0,0,0,-1,-1,127,0,0,1,0,0,0,0],51824": 597,
	*/
	var ret []BpfNetIOTrafficCounter
	for endpoint, trafficBytes := range rec.Data {
		ipPort := strings.Split(endpoint, ",")
		if len(ipPort) != 2 {
			continue
		}
		ipStr, portStr := strings.TrimSuffix(strings.TrimPrefix(ipPort[0], "["), "]"), ipPort[1]
		port, _ := strconv.Atoi(portStr)

		ret = append(ret, BpfNetIOTrafficCounter{
			IP:          []byte{},
			Port:        port,
			ByteCounter: trafficBytes,
		})
	}
}

type BpfTracer struct {
	PID                 int
	SamplingIntervalSec int
	FDBytesRead         map[int]int
	FDBytesWritten      map[int]int
}

func NewBpfTracer(pid int, samplingIntervalSec int) *BpfTracer {
	return &BpfTracer{
		PID:                 pid,
		SamplingIntervalSec: samplingIntervalSec,
		FDBytesRead:         make(map[int]int),
		FDBytesWritten:      make(map[int]int),
	}
}

func (bpf *BpfTracer) Start() error {
	bpf.FDBytesRead = make(map[int]int)
	bpf.FDBytesWritten = make(map[int]int)
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
tracepoint:tcp:tcp_probe {
    @tcp_src[args->saddr, args->sport] += args->data_len;
    @tcp_dest[args->daddr, args->dport] += args->data_len;
}
interval:s:%d {
    print(@read_fd); print(@write_fd); print(@tcp_src); print(@tcp_dest);
    clear(@read_fd); clear(@write_fd); clear(@tcp_src); clear(@tcp_dest);
}
	`, bpf.PID, bpf.PID, bpf.PID, bpf.PID, bpf.SamplingIntervalSec)
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
	go func() {
		stdoutReader := bufio.NewReader(stdout)
		for {
			line, err := stdoutReader.ReadString('\n')
			if err != nil {
				return
			}
			var rec BpfFDRecord
			if err := json.Unmarshal([]byte(line), &rec); err != nil {
				continue
			}
			if rec.Type == "map" && rec.Data != nil {
				if read := rec.Data["@read_fd"]; read != nil {
					for fd, count := range read {
						fdNumber, err := strconv.Atoi(fd)
						if err != nil {
							break
						}
						bpf.FDBytesRead[fdNumber] = count
					}
				} else if written := rec.Data["@write_fd"]; written != nil {
					for fd, count := range written {
						fdNumber, err := strconv.Atoi(fd)
						if err != nil {
							break
						}
						bpf.FDBytesWritten[fdNumber] = count
					}
				}
			}
		}
	}()
	return cmd.Wait()
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
		fileName, exists := fdPaths[fd]
		if !exists {
			continue
		}
		if _, exists := ret.ByName[fileName]; !exists {
			ret.ByName[fileName] = &FileIOCounter{Name: fileName}
		}
		ret.ByName[fileName].ReadBytes = read
	}
	for fd, written := range bpf.FDBytesWritten {
		fileName, exists := fdPaths[fd]
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
