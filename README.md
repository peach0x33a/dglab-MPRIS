# dglab-MPRIS

将 DG-LAB 设备桥接到 Linux 的 MPRIS/D-Bus 媒体控制生态。

本项目启动后会：

- 在本机开启一个 WebSocket 服务，供 DG-LAB 手机 APP 扫码连接
- 将 A/B 两个通道分别注册为 MPRIS 播放器（`org.mpris.MediaPlayer2.DGLab_A` / `org.mpris.MediaPlayer2.DGLab_B`）
- 把媒体控制动作（播放/暂停/停止/上一首/下一首/拖动进度）映射为 DG-LAB 强度与波形指令
- 提供终端 TUI，实时展示连接状态、通道强度、硬上限和当前波形

## 功能特性

- 双通道独立控制（A/B）
- MPRIS 兼容控制：
  - Play / Pause / Stop / PlayPause
  - Next / Previous（切换预设波形）
  - Seek / SetPosition（按秒映射强度）
- 自动心跳保活
- 启动时打印配对二维码（终端半块二维码）
- 支持交互式选择绑定网卡地址，也支持命令行直接指定 `-host`
- 预置 16 组波形（来自 `DG_WAVES_V2_V3_simple.js`）
- 日志写入 `dglab-mpris.log`

## 运行环境

- Linux（需要 Session D-Bus 与 MPRIS 环境，KDE Plasma 等桌面可直接识别）
- Go 1.25+
- DG-LAB 手机 APP（用于扫码连接）

> 说明：项目核心依赖见 `go.mod`，包括 `github.com/godbus/dbus/v5`、`github.com/gorilla/websocket`、`github.com/charmbracelet/bubbletea` 等。

## 快速开始

### 1) 拉取依赖并运行

```bash
go mod tidy
go run .
```

启动后程序会：

1. 列出可用网卡地址并让你选择监听地址（若未传 `-host`）
2. 输出 WebSocket 地址
3. 在终端打印二维码
4. 等待 DG-LAB APP 扫码配对
5. 配对成功后注册两个 MPRIS 服务并进入 TUI 界面

### 2) 使用命令行参数

```bash
go run . -host 192.168.1.100 -port 9999
```

参数：

- `-host`：二维码中使用的主机地址，同时作为默认绑定地址（不传则进入交互选择）
- `-port`：WebSocket 监听端口，默认 `9999`

## 使用说明

### 配对流程

- 启动程序后，用 DG-LAB APP 扫描终端中的二维码
- APP 连接成功后，程序会将目标设备标记为已配对
- 此时系统中应出现两个 MPRIS 播放器（A/B）

### 在桌面媒体控件中控制

你可以在系统媒体控件或任意 MPRIS 客户端中操作：

- **Play**：将当前通道强度恢复到上次非零值（并受硬上限约束）
- **Pause/Stop**：强度归零
- **Next/Previous**：切换预设波形（循环切换，包含“手动模式”）
- **Seek/SetPosition**：以“秒”为单位映射到强度值

### TUI 显示内容

- 服务监听地址
- APP 连接状态与设备 ID
- MPRIS 注册状态
- A/B 通道实时强度进度条（当前值/硬上限）
- A/B 当前波形名称

按 `q` 或 `Ctrl+C` 退出。

## 日志与故障排查

日志文件：

- `dglab-mpris.log`

常见问题：

1. **看不到 MPRIS 播放器**
   - 确认在 Linux 图形会话内运行，且 `DBUS_SESSION_BUS_ADDRESS` 可用
   - 确认程序已完成 APP 配对（配对后才注册 MPRIS）

2. **APP 扫码后无法连接**
   - 确认手机与运行程序的机器在同一局域网
   - 确认监听地址/端口可达（防火墙未阻挡）
   - 当绑定 `0.0.0.0` 时，程序会尝试选首个非回环 IP 生成二维码地址

3. **强度变化不同步**
   - 查看日志中 WebSocket 收发内容
   - 确认 APP 未断开（断开后状态会重置）

## 开发说明

### 代码结构

- `main.go`：
  - WebSocket 服务与配对流程
  - DG-LAB 消息编解码
  - MPRIS 接口实现与 D-Bus 属性更新
  - 波形循环下发与心跳
- `tui.go`：Bubble Tea 终端界面
- `DG_WAVES_V2_V3_simple.js`：波形源数据
- `replace_waves.py`：将 JS 波形数据批量替换到 `main.go` 中的辅助脚本

### 波形数据更新

若你修改了 `DG_WAVES_V2_V3_simple.js`，可使用：

```bash
python3 replace_waves.py
```

该脚本会重建 `waveData` 与 `waveNames` 并写回 `main.go` 的波形区块。

## 安全提示

本项目用于设备控制。请根据自身设备与使用场景，合理设置强度与上限，避免不适或风险。
