package main

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const (
	MaxPanel = 3
)

type RefreshMessage time.Time

func refreshAfter(interval time.Duration) tea.Cmd {
	return tea.Tick(interval, func(t time.Time) tea.Msg { return RefreshMessage(t) })
}

type MainModel struct {
	FocusIndex    int
	ProcInfo      *ProcInfo
	OverviewModel *OverviewModel
	FileModel     *FileModel
	NetModel      *NetModel
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
				model.FocusIndex = 0
			}
		}
	}
	_, overviewUpdate := model.OverviewModel.Update(msg)
	_, fileStatsUpdate := model.FileModel.Update(msg)
	return model, tea.Batch(overviewUpdate, fileStatsUpdate)
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

	return lipgloss.JoinVertical(lipgloss.Top,
		lipgloss.JoinHorizontal(lipgloss.Left, overview, fileStats, netStats),
		lipgloss.JoinHorizontal(lipgloss.Left, netStats),
	)
}
