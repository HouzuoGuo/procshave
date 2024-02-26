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
	model := &MainModel{
		ProcInfo:       procInfo,
		OverviewModel:  NewOverviewModel(pid, procInfo, 1*time.Second),
		FileStatsModel: NewFileStatsModel(pid, procInfo, BPFSampleIntervalSec),
	}
	if _, err := tea.NewProgram(model, tea.WithAltScreen()).Run(); err != nil {
		log.Panic(err)
	}
}
