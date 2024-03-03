package main

import (
	"fmt"
	"log"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type BlkdevModel struct {
	PID       int
	BPF       *BpfTracer
	Proc      *ProcInfo
	TermWidth int
}

func NewBlkdevModel(pid int, procInfo *ProcInfo, bpf *BpfTracer) *BlkdevModel {
	return &BlkdevModel{PID: pid, Proc: procInfo, BPF: bpf}
}

func (model *BlkdevModel) Init() tea.Cmd {
	return nil
}

func (model *BlkdevModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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

func (model *BlkdevModel) GetRegularStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Width(model.TermWidth/2-2).Height(15).Align(lipgloss.Left, lipgloss.Top).
		BorderStyle(lipgloss.RoundedBorder())
}

func (model *BlkdevModel) GetFocusedStyle() lipgloss.Style {
	return lipgloss.NewStyle().Inherit(model.GetRegularStyle()).
		BorderForeground(lipgloss.Color(FocusedBorderForeground)).
		BorderBackground(lipgloss.Color(FocusedBorderBackground))
}

func (model *BlkdevModel) View() string {
	var ret string
	ret += genericLabel.Render("Block device IO activities") + "\n"
	blkdevs := model.BPF.BlockIOSummary(model.Proc.DiskStats)
	if len(blkdevs.ByDuration) == 0 {
		ret += "No data yet."
		return ret
	}
	for i, blkdev := range blkdevs.ByDuration {
		if i == 12 {
			break
		}
		sectors := fmt.Sprintf("%d sectors(%s)",
			blkdev.SectorCount/model.BPF.SamplingIntervalSec,
			(blkdev.IODuration / time.Duration(model.BPF.SamplingIntervalSec)).Round(1*time.Millisecond))
		ret += fmt.Sprintf("%-26s %-18s /s", PathCaption(blkdev.DeviceName, 25), sectors)
	}
	return ret
}
