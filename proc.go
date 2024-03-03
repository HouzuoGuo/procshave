package main

import (
	"fmt"
	"sync"
	"time"

	"github.com/prometheus/procfs"
	"github.com/prometheus/procfs/blockdevice"
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
	DiskStats    map[string]blockdevice.Diskstats

	Mutex *sync.RWMutex
}

func NewProcInfo(pid int, bpfSampleIntervalSec int) *ProcInfo {
	ret := &ProcInfo{PID: pid, Mutex: new(sync.RWMutex)}
	fs, _ := procfs.NewDefaultFS()
	stat, _ := fs.Stat()
	ret.Uptime = time.Since(time.Unix(int64(stat.BootTime), 0))
	ret.TargetInfo = NewProcessInfo(ret.PID)
	ret.ParentInfo = NewProcessInfo(ret.TargetInfo.MainStat.PPID)
	ret.TTYGroupInfo = NewProcessInfo(ret.TargetInfo.MainStat.TPGID)
	ret.GroupInfo = NewProcessInfo(ret.TargetInfo.MainStat.PGRP)
	ret.SessionInfo = NewProcessInfo(ret.TargetInfo.MainStat.Session)
	ret.Refresh()
	return ret
}

func (info *ProcInfo) Refresh() {
	info.Mutex.Lock()
	defer info.Mutex.Unlock()
	fs, _ := procfs.NewDefaultFS()
	stat, _ := fs.Stat()
	info.Uptime = time.Since(time.Unix(int64(stat.BootTime), 0))
	info.TargetInfo.Refresh()
	info.ParentInfo.Refresh()
	info.TTYGroupInfo.Refresh()
	info.GroupInfo.Refresh()
	info.SessionInfo.Refresh()

	blockdev, _ := blockdevice.NewDefaultFS()
	diskStats, _ := blockdev.ProcDiskstats()
	info.DiskStats = make(map[string]blockdevice.Diskstats)
	for _, disk := range diskStats {
		info.DiskStats[fmt.Sprintf("%d:%d", disk.MajorNumber, disk.MinorNumber)] = disk
	}
}
