package main

import (
	"fmt"
	"log"
	"strconv"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	titleStyle = func() lipgloss.Style {
		b := lipgloss.RoundedBorder()
		b.Right = "├"
		return lipgloss.NewStyle().BorderStyle(b).Padding(0, 1)
	}()

	// white adapted to the terminal emulator's theme
	genericLabel = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#000000"))
	// green adapted to the terminal emulator's theme
	overviewRunningTaskStyle = lipgloss.NewStyle().Background(lipgloss.Color("#00ff00"))
	// blue adapted to the terminal emulator's theme
	overviewSleepingTaskStyle = lipgloss.NewStyle().Background(lipgloss.Color("#0000ff"))
	// red adapted to the terminal emulator's theme
	overviewOtherTaskStyle = lipgloss.NewStyle().Background(lipgloss.Color("##ff0000"))
	overviewModelStyle     = lipgloss.NewStyle().Width(50).Height(5).Align(lipgloss.Left, lipgloss.Top).BorderStyle(lipgloss.NormalBorder())
	oocusedOverviewStyle   = lipgloss.NewStyle().Inherit(overviewModelStyle).
				BorderStyle(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("228")).BorderBackground(lipgloss.Color("63"))
)

type RefreshMessage time.Time

func refreshAfter(interval time.Duration) tea.Cmd {
	return tea.Tick(interval, func(t time.Time) tea.Msg { return RefreshMessage(t) })
}

type OverviewModel struct {
	PID         int
	RefreshRate time.Duration
	Overview    *Overview
}

func NewOverviewModel(pid int, refreshRate time.Duration) *OverviewModel {
	return &OverviewModel{
		PID:         pid,
		RefreshRate: refreshRate,
		Overview:    NewOverview(pid),
	}
}

func (model *OverviewModel) Init() tea.Cmd {
	model.Overview.Refresh()
	return refreshAfter(model.RefreshRate)
}

func (model *OverviewModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case RefreshMessage:
		model.Overview.Refresh()
		return model, refreshAfter(model.RefreshRate)
	case tea.KeyMsg:
		switch msg.String() {
		case tea.KeyEnter.String():
			log.Printf("@@@@@ got enter")
		}
	}
	return model, nil
}

func (model *OverviewModel) renderHierarchy() string {
	var ret string
	session := model.Overview.SessionInfo
	tty := model.Overview.TTYGroupInfo
	parent := model.Overview.ParentInfo
	group := model.Overview.GroupInfo
	target := model.Overview.TargetInfo
	ret += fmt.Sprintf("%s %s\n", genericLabel.Render("Exe:  "), target.MainExec)
	ret += fmt.Sprintf("%s %s\n", genericLabel.Render("Cwd:  "), target.MainCWD)
	ret += fmt.Sprintf("%s %v\n\n", genericLabel.Render("Since:"), time.Duration(model.Overview.Uptime-time.Duration(model.Overview.TargetInfo.StartSecSinceBoot)*time.Second).Round(1*time.Second))
	ret += fmt.Sprintf("%s %s %d %s (%s:%s)\n",
		genericLabel.Render("┌Session   "), renderTaskState(session.MainStat.State, session.MainStat.State),
		session.MainStat.PID, session.MainComm, session.MainStatus.UIDs[0], session.MainStatus.GIDs[0])
	if tty.PID > 0 {
		ret += fmt.Sprintf("%s %s %d %s (%s:%s)\n",
			genericLabel.Render("├─TTY group"),
			renderTaskState(tty.MainStat.State, tty.MainStat.State), tty.MainStat.PID, tty.MainComm,
			tty.MainStatus.UIDs[0], tty.MainStatus.GIDs[0])
	} else {
		ret += fmt.Sprintf("%s\n", genericLabel.Render("├─TTY group not used"))
	}
	if parent.MainStat.PID < group.MainStat.PID {
		ret += fmt.Sprintf("%s %s %d %s (%s:%s)\n",
			genericLabel.Render("└┬Parent   "), renderTaskState(parent.MainStat.State, parent.MainStat.State),
			parent.MainStat.PID, parent.MainComm, parent.MainStatus.UIDs[0], parent.MainStatus.GIDs[0])
		ret += fmt.Sprintf("%s %s %d %s (%s:%s)\n",
			genericLabel.Render(" └┬Group   "), renderTaskState(group.MainStat.State, group.MainStat.State),
			group.MainStat.PID, group.MainComm, group.MainStatus.UIDs[0], group.MainStatus.GIDs[0])
	} else {
		ret += fmt.Sprintf("%s %s %d %s (%s:%s)\n",
			genericLabel.Render("└┬Group    "), renderTaskState(group.MainStat.State, group.MainStat.State),
			group.MainStat.PID, group.MainComm, group.MainStatus.UIDs[0], group.MainStatus.GIDs[0])
		ret += fmt.Sprintf("%s %s %d %s (%s:%s)\n",
			genericLabel.Render(" └┬Parent  "), renderTaskState(parent.MainStat.State, parent.MainStat.State),
			parent.MainStat.PID, parent.MainComm, parent.MainStatus.UIDs[0], parent.MainStatus.GIDs[0])
	}
	ret += fmt.Sprintf("%sTarget > %s %d %s (%s:%s)\n",
		genericLabel.Render("  └"), renderTaskState(target.MainStat.State, target.MainStat.State),
		target.MainStat.PID, target.MainComm, target.MainStatus.UIDs[0], target.MainStatus.GIDs[0])
	return ret + "\n"
}

func renderTaskState(state, caption string) string {
	switch state {
	case "S":
		return overviewSleepingTaskStyle.Render(caption)
	case "R":
		return overviewRunningTaskStyle.Render(caption)
	default:
		return overviewOtherTaskStyle.Render(caption)
	}
}

func (model *OverviewModel) renderResourceUsage() string {
	var ret string
	ret += fmt.Sprintf("%s", genericLabel.Render("Threads: "))
	for i, stat := range model.Overview.TargetInfo.Stat {
		ret += renderTaskState(stat.State, strconv.Itoa(i)) + " "
	}
	ret += "\n"
	return ret
}

func (model *OverviewModel) View() string {
	var ret string
	ret += model.renderHierarchy()
	ret += model.renderResourceUsage()
	return ret
}

type MainModel struct {
	FocusIndex    int
	OverviewModel *OverviewModel
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
		}
	}
	_, overviewUpdate := model.OverviewModel.Update(msg)
	return model, tea.Batch(overviewUpdate)
}

func (model *MainModel) View() string {
	overview := overviewModelStyle.Render(model.OverviewModel.View())
	if model.FocusIndex == 0 {
		overview = oocusedOverviewStyle.Render(model.OverviewModel.View())
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, overview)
}
