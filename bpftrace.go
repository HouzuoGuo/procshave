package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os/exec"
	"regexp"
	"sort"
	"strconv"
	"strings"
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
}

func TcpTrafficFromBpfMap(bpfMap map[string]int) []BpfNetIOTrafficCounter {
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
		})

	}
	return ret
}

type BpfTracer struct {
	PID                    int
	SamplingIntervalSec    int
	FDBytesRead            map[int]int
	FDBytesWritten         map[int]int
	TcpTrafficSources      []BpfNetIOTrafficCounter
	TcpTrafficDestinations []BpfNetIOTrafficCounter
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
			var rec BpfMapRecord
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
				} else if tcpSrc := rec.Data["@tcp_src"]; tcpSrc != nil {
					bpf.TcpTrafficSources = TcpTrafficFromBpfMap(tcpSrc)
				} else if tcpDest := rec.Data["@tcp_dest"]; tcpDest != nil {
					bpf.TcpTrafficDestinations = TcpTrafficFromBpfMap(tcpDest)
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
