package main

import (
	"fmt"
	"log"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type FileModel struct {
	PID       int
	BPF       *BpfTracer
	Proc      *ProcInfo
	TermWidth int
}

func NewFileModel(pid int, procInfo *ProcInfo, bpf *BpfTracer) *FileModel {
	return &FileModel{PID: pid, Proc: procInfo, BPF: bpf}
}

func (model *FileModel) Init() tea.Cmd {
	return nil
}

func (model *FileModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case tea.KeyEnter.String():
			log.Printf("@@@@@ got enter")
		}
	case tea.WindowSizeMsg:
		model.TermWidth = msg.Width
	}
	return model, nil
}

func (model *FileModel) GetRegularStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Width(model.TermWidth/2-2).Height(15).Align(lipgloss.Left, lipgloss.Top).
		BorderStyle(lipgloss.RoundedBorder())
}

func (model *FileModel) GetFocusedStyle() lipgloss.Style {
	return lipgloss.NewStyle().Inherit(model.GetRegularStyle()).
		BorderForeground(lipgloss.Color(FocusedBorderForeground)).
		BorderBackground(lipgloss.Color(FocusedBorderBackground))
}

func (model *FileModel) View() string {
	var ret string
	ret += genericLabel.Render("File IO activities") + "\n"
	files := model.BPF.FileIOSummary(model.Proc.TargetInfo.FDPath)
	if len(files.ByRate) == 0 {
		ret += "No data yet."
		return ret
	}
	for i, file := range files.ByRate {
		if i == 12 {
			break
		}
		ret += fmt.Sprintf("%-27s R %-8s W %-8s\n",
			PathCaption(file.Name, 25),
			IORateCaption(file.ReadBytes/model.BPF.SamplingIntervalSec),
			IORateCaption(file.WrittenBytes/model.BPF.SamplingIntervalSec))
	}
	return ret
}
