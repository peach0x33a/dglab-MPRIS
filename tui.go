package main

import (
	"bytes"
	"fmt"
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

// pairedMsg 表示配对成功
type pairedMsg struct{}

func waitForPairing(ch <-chan struct{}) tea.Cmd {
	return func() tea.Msg {
		<-ch
		return pairedMsg{}
	}
}

func waitForDisconnect(ch <-chan struct{}) tea.Cmd {
	return func() tea.Msg {
		<-ch
		return qrDisconnectedMsg{}
	}
}

// SelectAddressModel 地址选择模型
type SelectAddressModel struct {
	addresses        []ifaceAddr
	selected         int
	addrCh           chan<- string // 用于通知 main.go 启动服务器
	currentView      string        // "select" 或 "qr"
	qrContent        string
	qrURL            string
	wsAddr           string
	port             int
	clientID         string
	qrDisconnectedCh chan struct{} // 传递给 AppModel
}

// NewSelectAddressModel 创建地址选择模型
func NewSelectAddressModel(addrs []ifaceAddr, addrCh chan<- string, port int, clientID string, qrDisconnectedCh chan struct{}) *SelectAddressModel {
	return &SelectAddressModel{
		addresses:        addrs,
		selected:         0,
		addrCh:           addrCh,
		currentView:      "select",
		port:             port,
		clientID:         clientID,
		qrDisconnectedCh: qrDisconnectedCh,
	}
}

func (m *SelectAddressModel) Init() tea.Cmd {
	return nil
}

func (m *SelectAddressModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		}

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
				// 恢复 QR URL: 包含 path 中的 clientID
				m.qrURL = fmt.Sprintf("https://www.dungeon-lab.com/app-download.php#DGLAB-SOCKET#ws://%s:%d/%s", qrHost, m.port, m.clientID)

				var buf bytes.Buffer
				qrterminal.GenerateHalfBlock(m.qrURL, qrterminal.L, &buf)
				m.qrContent = buf.String()

				// 保存二维码图片
				qrFilename := "qrcode.png"
				_ = qrcode.WriteFile(m.qrURL, qrcode.Medium, 256, qrFilename)

				m.currentView = "qr"

				// 通知 main.go 启动服务器
				if m.addrCh != nil {
					m.addrCh <- selected
				}

				// 开始等待配对成功的信号
				return m, waitForPairing(boundCh)
			}
		}
	case pairedMsg:
		// 配对成功，切换到主应用界面
		appModel := NewAppModel(m.qrContent, m.qrDisconnectedCh, m)
		return appModel, appModel.Init()
	}
	return m, nil
}

func (m *SelectAddressModel) getSelectedAddress() string {
	if m.selected < len(m.addresses) {
		return m.addresses[m.selected].IP
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
	qrContent        string              // 二维码内容
	qrDisconnectedCh <-chan struct{}     // 接收连接断开通知
	parentModel      *SelectAddressModel // 用于断开时返回上级模型
}

// NewAppModel 创建 TUI 模型
func NewAppModel(qrContent string, qrDisconnectedCh <-chan struct{}, parent *SelectAddressModel) AppModel {
	return AppModel{
		qrContent:        qrContent,
		qrDisconnectedCh: qrDisconnectedCh,
		parentModel:      parent,
	}
}

func (m AppModel) Init() tea.Cmd {
	var cmds []tea.Cmd
	cmds = append(cmds, tick())
	if m.qrDisconnectedCh != nil {
		cmds = append(cmds, waitForDisconnect(m.qrDisconnectedCh))
	}
	return tea.Batch(cmds...)
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
		// 断开连接时，返回到父模型的二维码界面，并重新开始等待配对信号
		m.parentModel.currentView = "qr"
		return m.parentModel, tea.Batch(tick(), waitForPairing(boundCh))
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

	// AppModel 仅用于已连接状态，断开时的二维码显示已交由 SelectAddressModel 处理

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
