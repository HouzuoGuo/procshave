package main

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const (
	MaxPanel = 2
)

type RefreshMessage time.Time

func refreshAfter(interval time.Duration) tea.Cmd {
	return tea.Tick(interval, func(t time.Time) tea.Msg { return RefreshMessage(t) })
}

type MainModel struct {
	FocusIndex     int
	ProcInfo       *ProcInfo
	OverviewModel  *OverviewModel
	FileStatsModel *FileIOModel
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
	_, fileStatsUpdate := model.FileStatsModel.Update(msg)
	return model, tea.Batch(overviewUpdate, fileStatsUpdate)
}

func (model *MainModel) View() string {
	model.ProcInfo.Mutex.RLock()
	defer model.ProcInfo.Mutex.RUnlock()

	overview := overviewModelStyle.Render(model.OverviewModel.View())
	if model.FocusIndex == 0 {
		overview = oocusedOverviewStyle.Render(model.OverviewModel.View())
	}

	fileStats := fileStatsStyle.Render(model.FileStatsModel.View())
	if model.FocusIndex == 1 {
		fileStats = focusedFileStatsStyle.Render(model.FileStatsModel.View())
	}

	return lipgloss.JoinHorizontal(lipgloss.Top, overview, fileStats)
}
