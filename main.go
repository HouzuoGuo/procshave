package main

import (
	"flag"
	"log"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	var pid int
	flag.IntVar(&pid, "p", 1, "The process ID to monitor")
	flag.Parse()

	model := &MainModel{
		OverviewModel: NewOverviewModel(pid, 1*time.Second),
	}
	if _, err := tea.NewProgram(model, tea.WithAltScreen()).Run(); err != nil {
		log.Panic(err)
	}
}
