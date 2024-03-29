package main

import (
	"fmt"

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
		Width(model.TermWidth/2-2).Height(15).Align(lipgloss.Left, lipgloss.Top).
		BorderStyle(lipgloss.RoundedBorder())
}

func (model *NetModel) GetFocusedStyle() lipgloss.Style {
	return lipgloss.NewStyle().Inherit(model.GetRegularStyle()).
		BorderForeground(lipgloss.Color(FocusedBorderForeground)).
		BorderBackground(lipgloss.Color(FocusedBorderBackground))
}

func (model *NetModel) View() string {
	var ret string
	ret += genericLabel.Render("TCP activities - incoming") + "\n"
	if len(model.BPF.TcpTrafficDestinations)+len(model.BPF.TcpTrafficSources) == 0 {
		ret += "No data yet.\n"
		return ret
	}
	for i, counter := range model.BPF.TcpTrafficDestinations {
		if i == 6 {
			break
		}
		ret += fmt.Sprintf("%-39s %-5d %s\n", counter.IP, counter.Port, IORateCaption(counter.ByteCounter/model.BPF.SamplingIntervalSec))
	}
	ret += genericLabel.Render("TCP activities - outgoing") + "\n"
	for i, counter := range model.BPF.TcpTrafficSources {
		if i == 6 {
			break
		}
		ret += fmt.Sprintf("%-39s %-5d %s\n", counter.IP, counter.Port, IORateCaption(counter.ByteCounter/model.BPF.SamplingIntervalSec))
	}
	return ret
}
