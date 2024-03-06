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
	var promMetricsAddr string
	flag.IntVar(&pid, "p", 1, "The process ID to monitor")
	flag.StringVar(&promMetricsAddr, "metricsaddr", "0.0.0.0:1619", "The host:port to start prometheus metrics server on")
	flag.Parse()

	procInfo := NewProcInfo(pid, BPFSampleIntervalSec)
	metrics := NewMetricsCollector()
	bpf := NewBpfTracer(pid, BPFSampleIntervalSec, metrics)
	model := &MainModel{
		ProcInfo:      procInfo,
		BpfTracer:     bpf,
		OverviewModel: NewOverviewModel(pid, procInfo, 1*time.Second),
		FileModel:     NewFileModel(pid, procInfo, bpf),
		NetModel:      NewNetModel(pid, procInfo, bpf),
		BlkdevModel:   NewBlkdevModel(pid, procInfo, bpf),
	}

	go func() {
		if err := model.BpfTracer.Start(); err != nil {
			log.Printf("bpftrace error: %+v", err)
		}
	}()
	if promMetricsAddr != "" {
		if err := metrics.Start(promMetricsAddr); err != nil {
			log.Printf("prometheus metrics web server errror: %v", err)
		}
	}
	if _, err := tea.NewProgram(model, tea.WithAltScreen()).Run(); err != nil {
		log.Panic(err)
	}
}
