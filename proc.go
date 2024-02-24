package main

import (
	"time"

	"github.com/prometheus/procfs"
	"github.com/tklauser/go-sysconf"
)

type ProcessCommand struct {
	ID         int
	Executable string
}

type ProcessInfo struct {
	PID            int
	ticksPerSecond int

	Threads           procfs.Procs
	Status            []procfs.ProcStatus
	Stat              []procfs.ProcStat
	StartSecSinceBoot int

	MainComm    string
	MainExec    string
	MainCmdline []string
	MainStat    procfs.ProcStat
	MainStatus  procfs.ProcStatus
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
			proc.MainCmdline, _ = thread.CmdLine()
			proc.MainExec, _ = thread.Executable()
			proc.MainStat = stat
			proc.MainStatus = status
			proc.StartSecSinceBoot = int(stat.Starttime) / proc.ticksPerSecond
		}
	}
}

func NewProcessInfo(pid int) *ProcessInfo {
	ret := &ProcessInfo{PID: pid}
	ticksPerSecond, err := sysconf.Sysconf(sysconf.SC_CLK_TCK)
	if err != nil {
		return ret
	}
	ret.ticksPerSecond = int(ticksPerSecond)
	ret.Refresh()
	return ret
}

type Overview struct {
	PID          int
	Uptime       time.Duration
	SessionInfo  *ProcessInfo
	TTYGroupInfo *ProcessInfo
	GroupInfo    *ProcessInfo
	ParentInfo   *ProcessInfo
	TargetInfo   *ProcessInfo
}

func NewOverview(pid int) *Overview {
	ret := &Overview{PID: pid}
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

func (overview *Overview) Refresh() {
	fs, _ := procfs.NewDefaultFS()
	stat, _ := fs.Stat()
	overview.Uptime = time.Since(time.Unix(int64(stat.BootTime), 0))
	overview.TargetInfo.Refresh()
	overview.ParentInfo.Refresh()
	overview.TTYGroupInfo.Refresh()
	overview.GroupInfo.Refresh()
	overview.SessionInfo.Refresh()
}
