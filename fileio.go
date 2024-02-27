package main

import (
	"fmt"
	"log"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	fileStatsStyle = lipgloss.NewStyle().
			Width(50).Height(12).Align(lipgloss.Left, lipgloss.Top).
			BorderStyle(lipgloss.RoundedBorder())
	focusedFileStatsStyle = lipgloss.NewStyle().Inherit(fileStatsStyle).
				BorderStyle(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("228")).
				BorderBackground(lipgloss.Color("63"))
)

type FileIOModel struct {
	PID                  int
	BPFSampleIntervalSec int
	Proc                 *ProcInfo
}

func NewFileStatsModel(pid int, procInfo *ProcInfo, bpfSampleIntervalSec int) *FileIOModel {
	return &FileIOModel{PID: pid, Proc: procInfo, BPFSampleIntervalSec: bpfSampleIntervalSec}
}

func (model *FileIOModel) Init() tea.Cmd {
	return nil
}

func (model *FileIOModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case tea.KeyEnter.String():
			log.Printf("@@@@@ got enter")
		}
	}
	return model, nil
}

func (model *FileIOModel) ioRateCaption(sum int) string {
	average := sum / model.BPFSampleIntervalSec
	if average > 1024*1048576 {
		gb := average / 1024 / 1048576
		if gb == 0 {
			gb = 1
		}
		return fmt.Sprintf("%4dGB/s", gb)
	} else if average > 1048576 {
		mb := average / 1048576
		if mb == 0 {
			mb = 1
		}
		return fmt.Sprintf("%4dMB/s", mb)
	} else if average > 1024 {
		kb := average / 1024
		if kb == 0 {
			kb = 1
		}
		return fmt.Sprintf("%4dKB/s", kb)
	} else {
		return fmt.Sprintf("%4d B/s", average)
	}
}

func (model *FileIOModel) View() string {
	var ret string
	ret += genericLabel.Render("File R/W IO estimates may be off by ~20%.") + "\n"
	files := model.Proc.FDStat.FileTrace(model.Proc.TargetInfo.FDPath)
	if len(files.ByRate) == 0 {
		ret += "No file activities."
		return ret
	}
	for _, file := range files.ByRate {
		ret += fmt.Sprintf("%s R %s W - %s\n", model.ioRateCaption(file.ReadBytes), model.ioRateCaption(file.WrittenBytes), file.Name)
	}
	return ret
}
