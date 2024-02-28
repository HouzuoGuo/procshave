package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os/exec"
	"sort"
	"strconv"
)

type FDTraceRecord struct {
	Type string                    `json:"type"`
	Data map[string]map[string]int `json:"data"`
}

type Bpftrace struct {
	PID int
}

func (bpf *Bpftrace) StartFileDescriptorIP(durationSec int) (*FDTrace, error) {
	code := fmt.Sprintf(`
tracepoint:syscalls:sys_enter_read /pid == %d/ {@fd[tid] = args->fd;}
tracepoint:syscalls:sys_exit_read /pid == %d && @fd[tid]/ {if (args->ret > 0) {@read_fd[@fd[tid]] += args->ret;} delete(@fd[tid]);}
tracepoint:syscalls:sys_enter_write /pid == %d/ {@fd[tid] = args->fd;}
tracepoint:syscalls:sys_exit_write /pid == %d && @fd[tid]/ {if (args->ret > 0) {@write_fd[@fd[tid]] += args->ret;} delete(@fd[tid]);}
	`, bpf.PID, bpf.PID, bpf.PID, bpf.PID)
	cmd := exec.Command("bpftrace", "-e", code, "-f", "json", "-c", "/usr/bin/sleep "+strconv.Itoa(durationSec))
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	stdoutReader := bufio.NewReader(stdout)
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	trace := &FDTrace{
		ReadBytesPerFD:    make(map[int]int),
		WrittenBytesPerFD: make(map[int]int),
	}
	go func() {
		for {
			line, err := stdoutReader.ReadString('\n')
			if err != nil {
				return
			}
			var rec FDTraceRecord
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
						trace.ReadBytesPerFD[fdNumber] = count
					}
				} else if written := rec.Data["@write_fd"]; written != nil {
					for fd, count := range written {
						fdNumber, err := strconv.Atoi(fd)
						if err != nil {
							break
						}
						trace.WrittenBytesPerFD[fdNumber] = count
					}
				}
			}
		}
	}()
	if err := cmd.Wait(); err != nil {
		return nil, err
	}
	return trace, nil
}

type FDTrace struct {
	ReadBytesPerFD    map[int]int
	WrittenBytesPerFD map[int]int
}

type FileIOCounter struct {
	Name                    string
	ReadBytes, WrittenBytes int
}

type FileTrace struct {
	ByName map[string]*FileIOCounter
	ByRate []*FileIOCounter
}

func (trace *FDTrace) FileTrace(fdPaths map[int]string) *FileTrace {
	ret := &FileTrace{
		ByName: make(map[string]*FileIOCounter),
		ByRate: []*FileIOCounter{},
	}
	for fd, read := range trace.ReadBytesPerFD {
		fileName, exists := fdPaths[fd]
		if !exists {
			continue
		}
		if _, exists := ret.ByName[fileName]; !exists {
			ret.ByName[fileName] = &FileIOCounter{Name: fileName}
		}
		ret.ByName[fileName].ReadBytes = read
	}
	for fd, written := range trace.WrittenBytesPerFD {
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
