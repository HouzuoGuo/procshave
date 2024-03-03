package main

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const (
	MaxPanel = 4
)

var (
	FocusedBorderForeground = lipgloss.Color("228")
	FocusedBorderBackground = lipgloss.Color("63")
)

type RefreshMessage time.Time

func refreshAfter(interval time.Duration) tea.Cmd {
	return tea.Tick(interval, func(t time.Time) tea.Msg { return RefreshMessage(t) })
}

func PathCaption(fullPath string, maxLength int) string {
	fullPath = strings.TrimSpace(fullPath)
	if len(fullPath) <= maxLength {
		return fullPath
	}
	half := maxLength/2 - 2
	firstHalf := fullPath[:half]
	secondHalf := fullPath[len(fullPath)-half:]
	return fmt.Sprintf("%s..%s", firstHalf, secondHalf)
}

func IORateCaption(rate int) string {
	if rate > 1024*1048576 {
		gb := rate / 1024 / 1048576
		if gb == 0 {
			gb = 1
		}
		return fmt.Sprintf("%dGB/s", gb)
	} else if rate > 1048576 {
		mb := rate / 1048576
		if mb == 0 {
			mb = 1
		}
		return fmt.Sprintf("%dMB/s", mb)
	} else if rate > 1024 {
		kb := rate / 1024
		if kb == 0 {
			kb = 1
		}
		return fmt.Sprintf("%dKB/s", kb)
	} else {
		return fmt.Sprintf("%dB/s", rate)
	}
}

type MainModel struct {
	FocusIndex    int
	ProcInfo      *ProcInfo
	OverviewModel *OverviewModel
	FileModel     *FileModel
	NetModel      *NetModel
	BlkdevModel   *BlkdevModel
	BpfTracer     *BpfTracer
}

func (model *MainModel) Init() tea.Cmd {
	return tea.Batch(model.OverviewModel.Init())
}

func (model *MainModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case tea.KeyCtrlC.String(), "q":
			return model, tea.Quit
		case tea.KeyTab.String():
			model.FocusIndex++
			if model.FocusIndex == MaxPanel {
				model.FocusIndex = 0
			}
		case tea.KeyShiftTab.String():
			model.FocusIndex--
			if model.FocusIndex == -1 {
				model.FocusIndex = MaxPanel - 1
			}
		}
	}
	_, overviewUpdate := model.OverviewModel.Update(msg)
	_, fileUpdate := model.FileModel.Update(msg)
	_, netUpdate := model.NetModel.Update(msg)
	_, blkdevUpdate := model.BlkdevModel.Update(msg)
	return model, tea.Batch(overviewUpdate, fileUpdate, netUpdate, blkdevUpdate)
}

func (model *MainModel) View() string {
	model.ProcInfo.Mutex.RLock()
	defer model.ProcInfo.Mutex.RUnlock()

	overview := model.OverviewModel.GetRegularStyle().Render(model.OverviewModel.View())
	if model.FocusIndex == 0 {
		overview = model.OverviewModel.GetFocusedStyle().Render(model.OverviewModel.View())
	}
	fileStats := model.FileModel.GetRegularStyle().Render(model.FileModel.View())
	if model.FocusIndex == 1 {
		fileStats = model.FileModel.GetFocusedStyle().Render(model.FileModel.View())
	}
	netStats := model.NetModel.GetRegularStyle().Render(model.NetModel.View())
	if model.FocusIndex == 2 {
		netStats = model.NetModel.GetFocusedStyle().Render(model.NetModel.View())
	}
	blkdevStats := model.BlkdevModel.GetRegularStyle().Render(model.BlkdevModel.View())
	if model.FocusIndex == 3 {
		blkdevStats = model.BlkdevModel.GetFocusedStyle().Render(model.BlkdevModel.View())
	}

	return lipgloss.JoinVertical(lipgloss.Top,
		lipgloss.JoinHorizontal(lipgloss.Left, overview, fileStats),
		lipgloss.JoinHorizontal(lipgloss.Left, netStats, blkdevStats),
	)
}
