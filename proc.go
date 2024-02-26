package main

import (
	"log"
	"sync"
	"time"

	"github.com/prometheus/procfs"
	"github.com/tklauser/go-sysconf"
)

type ProcessInfo struct {
	PID            int
	ticksPerSecond int

	Threads           procfs.Procs
	Status            []procfs.ProcStatus
	Stat              []procfs.ProcStat
	StartSecSinceBoot int
	FDPath            map[int]string

	MainComm   string
	MainExec   string
	MainCWD    string
	MainStat   procfs.ProcStat
	MainStatus procfs.ProcStatus
}

func (proc *ProcessInfo) Refresh() {
	proc.Threads, _ = procfs.AllThreads(proc.PID)
	proc.Status = make([]procfs.ProcStatus, 0, len(proc.Threads))
	proc.Stat = make([]procfs.ProcStat, 0, len(proc.Threads))
	for _, thread := range proc.Threads {
		stat, _ := thread.Stat()
		proc.Stat = append(proc.Stat, stat)
		status, _ := thread.NewStatus()
		proc.Status = append(proc.Status, status)
		if thread.PID == proc.PID {
			proc.MainComm, _ = thread.Comm()
			proc.MainExec, _ = thread.Executable()
			proc.MainStat = stat
			proc.MainCWD, _ = thread.Cwd()
			proc.MainStatus = status
			proc.StartSecSinceBoot = int(stat.Starttime) / proc.ticksPerSecond
			fdTargets, _ := thread.FileDescriptorTargets()
			fdNumbers, _ := thread.FileDescriptors()
			if len(fdTargets) == len(fdNumbers) {
				proc.FDPath = make(map[int]string)
				for i, fd := range fdNumbers {
					proc.FDPath[int(fd)] = fdTargets[i]
				}
			}
		}
	}
}

func NewProcessInfo(pid int) *ProcessInfo {
	ret := &ProcessInfo{
		PID:    pid,
		FDPath: make(map[int]string),
	}
	ticksPerSecond, err := sysconf.Sysconf(sysconf.SC_CLK_TCK)
	if err != nil {
		return ret
	}
	ret.ticksPerSecond = int(ticksPerSecond)
	ret.Refresh()
	return ret
}

type ProcInfo struct {
	PID          int
	Uptime       time.Duration
	SessionInfo  *ProcessInfo
	TTYGroupInfo *ProcessInfo
	GroupInfo    *ProcessInfo
	ParentInfo   *ProcessInfo
	TargetInfo   *ProcessInfo
	FDStat       *FDTrace

	Mutex *sync.RWMutex
}

func NewProcInfo(pid int, bpfSampleIntervalSec int) *ProcInfo {
	ret := &ProcInfo{PID: pid, FDStat: &FDTrace{}, Mutex: new(sync.RWMutex)}
	fs, _ := procfs.NewDefaultFS()
	stat, _ := fs.Stat()
	ret.Uptime = time.Since(time.Unix(int64(stat.BootTime), 0))
	ret.TargetInfo = NewProcessInfo(ret.PID)
	ret.ParentInfo = NewProcessInfo(ret.TargetInfo.MainStat.PPID)
	ret.TTYGroupInfo = NewProcessInfo(ret.TargetInfo.MainStat.TPGID)
	ret.GroupInfo = NewProcessInfo(ret.TargetInfo.MainStat.PGRP)
	ret.SessionInfo = NewProcessInfo(ret.TargetInfo.MainStat.Session)
	ret.Refresh()
	go func(pid int) {
		for {
			bpf := &Bpftrace{PID: pid}
			fdstat, err := bpf.StartFileDescriptorIP(bpfSampleIntervalSec)
			if err != nil {
				log.Printf("@@@@@@@@ bpftrace err: %v", err)
				break
			}
			ret.Mutex.Lock()
			ret.FDStat = fdstat
			ret.Mutex.Unlock()
		}
	}(pid)
	return ret
}

func (overview *ProcInfo) Refresh() {
	overview.Mutex.Lock()
	defer overview.Mutex.Unlock()
	fs, _ := procfs.NewDefaultFS()
	stat, _ := fs.Stat()
	overview.Uptime = time.Since(time.Unix(int64(stat.BootTime), 0))
	overview.TargetInfo.Refresh()
	overview.ParentInfo.Refresh()
	overview.TTYGroupInfo.Refresh()
	overview.GroupInfo.Refresh()
	overview.SessionInfo.Refresh()
}
