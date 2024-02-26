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

func (model *FileIOModel) View() string {
	var ret string
	ret += genericLabel.Render("File R/W IO estimates may be off by ~20%.") + "\n"
	files := model.Proc.FDStat.FileTrace(model.Proc.TargetInfo.FDPath)
	if len(files.ByRate) == 0 {
		ret += "No file activities."
		return ret
	}
	for _, file := range files.ByRate {
		if file.ReadBytes > 1024*1048576 {
			ret += fmt.Sprintf("%4dGB/s R %4dGB/s W - %s\n", file.ReadBytes/1048576/1024/model.BPFSampleIntervalSec, file.WrittenBytes/1048576/1024/model.BPFSampleIntervalSec, file.Name)
		} else if file.ReadBytes > 1048576 {
			ret += fmt.Sprintf("%4dMB/s R %4dMB/s W - %s\n", file.ReadBytes/1048576/model.BPFSampleIntervalSec, file.WrittenBytes/1048576/model.BPFSampleIntervalSec, file.Name)
		} else {
			ret += fmt.Sprintf("%4dKB/s R %4dKB/s W - %s\n", file.ReadBytes/1024/model.BPFSampleIntervalSec, file.WrittenBytes/1024/model.BPFSampleIntervalSec, file.Name)
		}
	}
	return ret
}
