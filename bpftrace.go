package main

import (
	"fmt"
	"io"
	"log"
	"os/exec"
	"strconv"
	"strings"
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
		tracepoint:syscalls:sys_enter_write /pid == %d/ { @write_fd[args->fd] += args->count; }
		tracepoint:syscalls:sys_enter_read /pid == %d/ { @read_fd[args->fd] += args->count; }
	`, bpf.PID, bpf.PID)
	log.Printf("@@@@@@ code: %v", code)
	log.Printf("cmd line: %v", strings.Join([]string{"/usr/bin/bpftrace", "-B", "line", "-e", code, "-f", "json", "-c", "/usr/bin/sleep " + strconv.Itoa(durationSec)}, " "))
	cmd := exec.Command("/usr/bin/bpftrace", "-B", "line", "-e", code, "-f", "json", "-c", "/usr/bin/sleep "+strconv.Itoa(durationSec))
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	trace := &FDTrace{
		ReadBytesPerFD:    make(map[int]int),
		WrittenBytesPerFD: make(map[int]int),
	}
	out, err := io.ReadAll(stdout)
	log.Printf("@@@@@@@ out: %s, err: %v", string(out), err)
	/*
		var rec FDTraceRecord
		decoder := json.NewDecoder(stdout)
		go func() {
			for {
				err := decoder.Decode(&rec)
				if errors.Is(err, io.EOF) {
					break
				}
				if err != nil {
					log.Printf("@@@@@ decode err: %v", err)
					continue
				}
				log.Printf("@@@@@ got rec: %+v", rec)
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
	*/
	if err := cmd.Wait(); err != nil {
		return nil, err
	}
	return trace, nil
}

type FDTrace struct {
	ReadBytesPerFD    map[int]int
	WrittenBytesPerFD map[int]int
}
