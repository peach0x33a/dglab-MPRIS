package main

import (
	"bytes"
	"fmt"
	"log"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	qrterminal "github.com/mdp/qrterminal/v3"
	qrcode "github.com/skip2/go-qrcode"
)

// UI 样式定义
var (
	mainColor     = lipgloss.Color("#ffe99d")
	titleStyle    = lipgloss.NewStyle().Foreground(mainColor).Bold(true).BorderStyle(lipgloss.RoundedBorder()).BorderForeground(mainColor).Padding(0, 1)
	subStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	labelStyle    = lipgloss.NewStyle().Foreground(mainColor).Bold(true)
	valueStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))
	successStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	warningStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
	barStyle      = lipgloss.NewStyle().Foreground(mainColor)
	waveStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("86"))
	selectedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("46")).Bold(true)
)

// tickMsg 用于定时刷新 UI
type tickMsg time.Time

func tick() tea.Cmd {
	return tea.Tick(time.Millisecond*500, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

// SelectAddressModel 地址选择模型
type SelectAddressModel struct {
	addresses   []ifaceAddr
	selected    int
	quitCh      chan<- string
	resultCh    <-chan string
	currentView string // "select" 或 "qr"
	qrContent   string
	wsAddr      string
	port        int
	clientID    string
	shouldQuit  bool // 标记是否应该退出（用于 nil quitCh 场景）
}

// NewSelectAddressModel 创建地址选择模型
func NewSelectAddressModel(addrs []ifaceAddr, quitCh chan<- string, port int, clientID string) *SelectAddressModel {
	return &SelectAddressModel{
		addresses:   addrs,
		selected:    0,
		quitCh:      quitCh,
		currentView: "select",
		port:        port,
		clientID:    clientID,
		shouldQuit:  false,
	}
}

func (m *SelectAddressModel) Init() tea.Cmd {
	return nil
}

func (m *SelectAddressModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.currentView == "select" {
			switch msg.String() {
			case "up", "k":
				if m.selected > 0 {
					m.selected--
				}
			case "down", "j":
				if m.selected < len(m.addresses) {
					m.selected++
				}
			case "enter":
				selected := m.getSelectedAddress()
				qrHost := selected
				if selected == "0.0.0.0" {
					for _, a := range m.addresses {
						if a.IP != "127.0.0.1" {
							qrHost = a.IP
							break
						}
					}
					if qrHost == "0.0.0.0" {
						qrHost = "localhost"
					}
				}
				m.wsAddr = fmt.Sprintf("%s:%d", selected, m.port)
				qrURL := fmt.Sprintf("https://www.dungeon-lab.com/app-download.php#DGLAB-SOCKET#ws://%s:%d/%s", qrHost, m.port, m.clientID)
				var buf bytes.Buffer
				qrterminal.GenerateHalfBlock(qrURL, qrterminal.L, &buf)
				m.qrContent = buf.String()
				// 保存二维码图片到当前目录
				qrFilename := fmt.Sprintf("qr_%s:%d.png", selected, m.port)
				if err := qrcode.WriteFile(qrURL, qrcode.Medium, 256, qrFilename); err != nil {
					log.Printf("保存二维码图片失败: %v", err)
				} else {
					log.Printf("二维码已保存: %s", qrFilename)
				}
				m.currentView = "qr"
			case "ctrl+c", "q":
				if m.quitCh != nil {
					m.quitCh <- ""
				} else {
					m.shouldQuit = true
				}
				return m, tea.Quit
			}
		} else if m.currentView == "qr" {
			switch msg.String() {
			case "enter":
				// 忽略，不要退出
			case "ctrl+c", "q":
				if m.quitCh != nil {
					m.quitCh <- ""
				} else {
					m.shouldQuit = true
				}
				return m, tea.Quit
			}
		}
	}
	return m, nil
}

func (m *SelectAddressModel) getSelectedAddress() string {
	if m.selected < len(m.addresses) {
		return m.addresses[m.selected].IP
	} else if m.selected == len(m.addresses) {
		return "0.0.0.0"
	}
	return "0.0.0.0"
}

func (m *SelectAddressModel) View() string {
	var b strings.Builder

	b.WriteString("\n")
	b.WriteString(titleStyle.Render("DG-LAB MPRIS 控制器 v2.0"))
	b.WriteString("\n\n")

	if m.currentView == "select" {
		b.WriteString(labelStyle.Render("请选择绑定地址（↑↓ 选择，Enter 确认）："))
		b.WriteString("\n\n")

		for i, a := range m.addresses {
			prefix := "  "
			style := subStyle
			if i == m.selected {
				prefix = "▶ "
				style = selectedStyle
			}
			b.WriteString(style.Render(fmt.Sprintf("%s[%d] %-12s %s", prefix, i+1, a.Name, a.IP)))
			b.WriteString("\n")
		}

		// All interfaces option
		prefix := "  "
		style := subStyle
		if m.selected == len(m.addresses) {
			prefix = "▶ "
			style = selectedStyle
		}
		b.WriteString(style.Render(fmt.Sprintf("%s[%d] %-12s %s", prefix, len(m.addresses)+1, "all", "0.0.0.0 (所有接口)")))
		b.WriteString("\n\n")

		b.WriteString(subStyle.Render("按 [q] 或 [ctrl+c] 退出"))
		b.WriteString("\n")
	} else if m.currentView == "qr" {
		b.WriteString(labelStyle.Render("绑定地址: "))
		b.WriteString(valueStyle.Render(m.getSelectedAddress()))
		b.WriteString("\n\n")

		b.WriteString(labelStyle.Render("服务监听: "))
		b.WriteString(valueStyle.Render("ws://" + m.wsAddr))
		b.WriteString("\n\n")

		b.WriteString(labelStyle.Render("请使用 DG-LAB APP 扫描以下二维码连接："))
		b.WriteString("\n\n")
		b.WriteString(m.qrContent)
		b.WriteString("\n\n")

		b.WriteString(warningStyle.Render("等待 APP 扫码连接..."))
		b.WriteString("\n\n")

		b.WriteString(subStyle.Render("按 [q] 或 [ctrl+c] 退出"))
		b.WriteString("\n")
	}

	return b.String()
}

// qrDisconnectedMsg 通知 TUI 显示二维码（连接已断开）
type qrDisconnectedMsg struct{}

// AppModel 表示我们的 TUI 状态
type AppModel struct {
	qrContent        string        // 二维码内容
	showQR           bool          // 是否显示二维码
	qrDisconnectedCh chan struct{} // 接收连接断开通知（仅在 TUI 运行时有效）
}

// NewAppModel 创建 TUI 模型
func NewAppModel(qrContent string, qrDisconnectedCh chan struct{}) AppModel {
	return AppModel{
		qrContent:        qrContent,
		showQR:           false,
		qrDisconnectedCh: qrDisconnectedCh,
	}
}

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
	case qrDisconnectedMsg:
		m.showQR = true
		return m, tick()
	}

	// 检查连接断开通知（仅在通道可用时检查）
	if m.qrDisconnectedCh != nil {
		select {
		case <-m.qrDisconnectedCh:
			m.showQR = true
			return m, tick()
		default:
		}
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

	// 连接断开后显示二维码
	if m.showQR && m.qrContent != "" {
		b.WriteString(warningStyle.Render("⚠️ APP 连接已断开，正在等待重新连接...\n\n"))
		b.WriteString(labelStyle.Render("请使用 DG-LAB APP 扫描以下二维码："))
		b.WriteString("\n\n")
		b.WriteString(m.qrContent)
		b.WriteString("\n\n")
		b.WriteString(labelStyle.Render("服务监听: "))
		b.WriteString(valueStyle.Render("ws://" + listenAddr))
		b.WriteString("\n\n")
		b.WriteString(subStyle.Render("按 [q] 或 [ctrl+c] 退出"))
		b.WriteString("\n")
		return b.String()
	}

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
