package main

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type NetModel struct {
	PID       int
	BPF       *BpfTracer
	Proc      *ProcInfo
	TermWidth int
}

func NewNetModel(pid int, procInfo *ProcInfo, bpf *BpfTracer) *NetModel {
	return &NetModel{PID: pid, Proc: procInfo, BPF: bpf}
}

func (model *NetModel) Init() tea.Cmd {
	return nil
}

func (model *NetModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		model.TermWidth = msg.Width
	}
	return model, nil
}

func (model *NetModel) GetRegularStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Width(model.TermWidth/2-2).Height(12).Align(lipgloss.Left, lipgloss.Top).
		BorderStyle(lipgloss.RoundedBorder())
}

func (model *NetModel) GetFocusedStyle() lipgloss.Style {
	return lipgloss.NewStyle().Inherit(model.GetRegularStyle()).
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("228")).
		BorderBackground(lipgloss.Color("63"))
}

func (model *NetModel) View() string {
	var ret string
	ret += genericLabel.Render("Network IO activities") + "\n"
	ret += "Initialising..."
	return ret
}
