package main

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// UI 样式定义
var (
	mainColor    = lipgloss.Color("#ffe99d")
	titleStyle   = lipgloss.NewStyle().Foreground(mainColor).Bold(true).BorderStyle(lipgloss.RoundedBorder()).BorderForeground(mainColor).Padding(0, 1)
	subStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	labelStyle   = lipgloss.NewStyle().Foreground(mainColor).Bold(true)
	valueStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))
	successStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	warningStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
	barStyle     = lipgloss.NewStyle().Foreground(mainColor)
	waveStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("86"))
)

// tickMsg 用于定时刷新 UI
type tickMsg time.Time

func tick() tea.Cmd {
	return tea.Tick(time.Millisecond*500, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

// AppModel 表示我们的 TUI 状态
type AppModel struct{}

func (m AppModel) Init() tea.Cmd {
	return tick()
}

func (m AppModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" || msg.String() == "q" {
			return m, tea.Quit
		}
	case tickMsg:
		return m, tick()
	}
	return m, nil
}

// progressBar 渲染简单的进度条
func progressBar(val, max int, width int) string {
	if max <= 0 {
		return strings.Repeat(" ", width)
	}
	pct := float64(val) / float64(max)
	if pct > 1.0 {
		pct = 1.0
	}
	filled := int(float64(width) * pct)
	empty := width - filled
	return barStyle.Render(strings.Repeat("█", filled)) + strings.Repeat("░", empty)
}

func (m AppModel) View() string {
	state.mu.Lock()
	paired := state.paired
	targetID := state.targetID
	listenAddr := state.listenAddr
	sA, sB := state.strengthA, state.strengthB
	mA, mB := state.maxA, state.maxB
	wA, wB := state.waveIdxA, state.waveIdxB
	mprisReady := state.mprisReady
	state.mu.Unlock()

	var b strings.Builder

	b.WriteString("\n")
	b.WriteString(titleStyle.Render("DG-LAB MPRIS 控制器 v2.0"))
	b.WriteString("\n\n")

	// 连接状态
	b.WriteString(labelStyle.Render("服务监听: "))
	b.WriteString(valueStyle.Render("ws://" + listenAddr))
	b.WriteString("\n")

	b.WriteString(labelStyle.Render("APP 状态: "))
	if paired {
		b.WriteString(successStyle.Render(fmt.Sprintf("✓ 已连接 (设备ID: %s)", targetID)))
	} else {
		b.WriteString(warningStyle.Render("等待通过 APP 扫码连接..."))
	}
	b.WriteString("\n\n")

	// MPRIS 状态
	b.WriteString(labelStyle.Render("MPRIS 状态: "))
	if mprisReady {
		b.WriteString(successStyle.Render("✓ 媒体组件已在系统中注册"))
	} else {
		b.WriteString(warningStyle.Render("等待注册..."))
	}
	b.WriteString("\n\n")

	// 通道控制区
	b.WriteString(labelStyle.Render("A 通道强度控制: "))
	if mA > 0 {
		b.WriteString(fmt.Sprintf("%s %d/%d", progressBar(sA, mA, 30), sA, mA))
	} else {
		b.WriteString(subStyle.Render("暂无数据"))
	}
	b.WriteString("\n")
	b.WriteString(labelStyle.Render("A 当前波形:     "))
	if wA >= 0 && wA < len(waveNames) {
		b.WriteString(waveStyle.Render(waveNames[wA]))
	} else {
		b.WriteString(subStyle.Render("手动模式 (无自动波形)"))
	}
	b.WriteString("\n\n")

	b.WriteString(labelStyle.Render("B 通道强度控制: "))
	if mB > 0 {
		b.WriteString(fmt.Sprintf("%s %d/%d", progressBar(sB, mB, 30), sB, mB))
	} else {
		b.WriteString(subStyle.Render("暂无数据"))
	}
	b.WriteString("\n")
	b.WriteString(labelStyle.Render("B 当前波形:     "))
	if wB >= 0 && wB < len(waveNames) {
		b.WriteString(waveStyle.Render(waveNames[wB]))
	} else {
		b.WriteString(subStyle.Render("手动模式 (无自动波形)"))
	}
	b.WriteString("\n\n")

	b.WriteString(subStyle.Render("按 [q] 或 [ctrl+c] 退出"))
	b.WriteString("\n")

	return b.String()
}
