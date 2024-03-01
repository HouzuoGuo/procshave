package main

import (
	"flag"
	"log"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

const (
	BPFSampleIntervalSec = 3
)

func main() {
	var pid int
	flag.IntVar(&pid, "p", 1, "The process ID to monitor")
	flag.Parse()

	procInfo := NewProcInfo(pid, BPFSampleIntervalSec)
	bpf := NewBpfTracer(pid, BPFSampleIntervalSec)
	model := &MainModel{
		ProcInfo:      procInfo,
		BpfTracer:     bpf,
		OverviewModel: NewOverviewModel(pid, procInfo, 1*time.Second),
		FileModel:     NewFileModel(pid, procInfo, bpf),
		NetModel:      NewNetModel(pid, procInfo, bpf),
	}

	go func() {
		if err := model.BpfTracer.Start(); err != nil {
			log.Printf("bpftrace error: %+v", err)
		}
	}()
	if _, err := tea.NewProgram(model, tea.WithAltScreen()).Run(); err != nil {
		log.Panic(err)
	}
}
