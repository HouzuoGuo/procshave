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
	genericLabel              = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#38598b"))
	overviewRunningTaskStyle  = lipgloss.NewStyle().Background(lipgloss.Color("#388e3c"))
	overviewSleepingTaskStyle = lipgloss.NewStyle().Background(lipgloss.Color("#385170"))
	overviewOtherTaskStyle    = lipgloss.NewStyle().Background(lipgloss.Color("#cd4439"))
)

type OverviewModel struct {
	PID         int
	RefreshRate time.Duration
	Proc        *ProcInfo
	TermWidth   int
}

func NewOverviewModel(pid int, procInfo *ProcInfo, refreshRate time.Duration) *OverviewModel {
	return &OverviewModel{
		PID:         pid,
		RefreshRate: refreshRate,
		Proc:        procInfo,
	}
}

func (model *OverviewModel) Init() tea.Cmd {
	model.Proc.Refresh()
	return refreshAfter(model.RefreshRate)
}

func (model *OverviewModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case RefreshMessage:
		model.Proc.Refresh()
		return model, refreshAfter(model.RefreshRate)
	case tea.WindowSizeMsg:
		model.TermWidth = msg.Width
	case tea.KeyMsg:
		switch msg.String() {
		case tea.KeyEnter.String():
			log.Printf("@@@@@ got enter")
		}
	}
	return model, nil
}

func (model *OverviewModel) GetRegularStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Width(model.TermWidth/2-2).Height(14).Align(lipgloss.Left, lipgloss.Top).
		BorderStyle(lipgloss.RoundedBorder())
}

func (model *OverviewModel) GetFocusedStyle() lipgloss.Style {
	return lipgloss.NewStyle().Inherit(model.GetRegularStyle()).
		BorderForeground(lipgloss.Color(FocusedBorderForeground)).
		BorderBackground(lipgloss.Color(FocusedBorderBackground))
}

func (model *OverviewModel) renderHierarchy() string {
	var ret string
	session := model.Proc.SessionInfo
	tty := model.Proc.TTYGroupInfo
	parent := model.Proc.ParentInfo
	group := model.Proc.GroupInfo
	target := model.Proc.TargetInfo
	ret += fmt.Sprintf("%s %s\n", genericLabel.Render("Exe:  "), target.MainExec)
	ret += fmt.Sprintf("%s %s\n", genericLabel.Render("Cwd:  "), target.MainCWD)
	ret += fmt.Sprintf("%s %v\n\n", genericLabel.Render("Since:"), time.Duration(model.Proc.Uptime-time.Duration(model.Proc.TargetInfo.StartSecSinceBoot)*time.Second).Round(1*time.Second))
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
	ret += fmt.Sprintf("%sTarget • %s %d %s (%s:%s)\n",
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
	if len(model.Proc.TargetInfo.Stat) <= 32 {
		ret += fmt.Sprintf("%s", genericLabel.Render("Threads: "))
		for i, stat := range model.Proc.TargetInfo.Stat {
			ret += renderTaskState(stat.State, strconv.Itoa(i)) + " "
		}
		ret += "\n"
	} else {
		var sleeping, running, other int
		for _, stat := range model.Proc.TargetInfo.Stat {
			switch stat.State {
			case "S":
				sleeping++
			case "R":
				running++
			default:
				other++
			}
		}
		ret += fmt.Sprintf("%s%s %s %s", genericLabel.Render("Threads: "),
			renderTaskState("R", fmt.Sprintf("%-4d running", running)),
			renderTaskState("S", fmt.Sprintf("%-4d sleeping", sleeping)),
			renderTaskState("other", fmt.Sprintf("%-3d other", other)),
		)
	}
	return ret
}

func (model *OverviewModel) View() string {
	var ret string
	ret += model.renderHierarchy()
	ret += model.renderResourceUsage()
	return ret
}
