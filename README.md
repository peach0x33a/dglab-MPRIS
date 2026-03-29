# dglab-MPRIS

将 DG-LAB 设备桥接到 Linux 的 MPRIS/D-Bus 媒体控制生态。

## ⚠️ 安全警告

**使用本项目造成的任何直接或间接伤害，包括但不限于身体不适、损伤等，均与本项目无关。**

请根据自身设备与使用场景，合理设置强度与上限，谨慎操作。

## 功能特性

- **双通道独立控制** - A/B 通道分别映射为独立播放器
- **MPRIS 兼容控制**
  - Play / Pause / Stop / PlayPause
  - Next / Previous（切换预设波形）
  - Seek / SetPosition（按秒映射强度）
- **16 组预置波形**（来源 [DG-LAB-OPENSOURCE](https://github.com/DG-LAB-OPENSOURCE/DG-LAB-OPENSOURCE/blob/main/socket/DG_WAVES_V2_V3_simple.js)）
- **终端 TUI** - 实时展示连接状态、通道强度、硬上限和当前波形
- **自动心跳保活**
- **二维码配对** - 启动时打印配对二维码

## 环境要求

- Linux（需 Session D-Bus 与 MPRIS 环境，KDE Plasma 等桌面可直接识别）
- Go 1.25+
- DG-LAB 手机 APP（扫码连接用）

## 兼容性提示

仅在开发者环境下测试通过：

| 组件     | 环境                       |
| -------- | -------------------------- |
| OS       | Fedora Linux 43 x86_64     |
| 桌面环境 | KDE Plasma 6.6.3 (Wayland) |
| 内核     | Linux 6.19.8               |
| Shell    | zsh 5.9                    |

## 快速开始

### 构建与运行

```bash
go mod tidy
go run .
```

或使用 Makefile：

```bash
make
./dglab-MPRIS
```

### 命令行参数

```bash
go run . -host 192.168.1.100 -port 9999
```

| 参数    | 默认值 | 说明                         |
| ------- | ------ | ---------------------------- |
| `-host` | -      | 监听地址，二维码中的主机地址 |
| `-port` | 9999   | WebSocket 监听端口           |

### 配置文件

除命令行参数外，也可通过 `config.yaml` 配置：

```yaml
host: 192.168.1.100
port: 9999
```

命令行参数优先级高于配置文件。

### 配对流程

1. 启动程序，选择或指定监听地址
2. 用 DG-LAB APP 扫描终端二维码
3. 连接成功后，程序注册两个 MPRIS 服务并进入 TUI

### 媒体控制映射

| 媒体控制   | DG-LAB 操作                      |
| ---------- | -------------------------------- |
| Play       | 恢复上次非零强度（受硬上限约束） |
| Pause/Stop | 强度归零                         |
| Next/Prev  | 切换预设波形（循环，含手动模式） |
| Seek       | 按秒数映射为强度值               |

### TUI 界面

- 服务监听地址
- APP 连接状态与设备 ID
- MPRIS 注册状态
- A/B 通道实时强度进度条
- 当前波形名称

按 `q` 或 `Ctrl+C` 退出。

## 项目结构

| 文件                       | 说明                                             |
| -------------------------- | ------------------------------------------------ |
| `main.go`                  | WebSocket 服务、配对、MPRIS 实现、波形下发、心跳 |
| `tui.go`                   | Bubble Tea 终端界面                              |
| `config.go`                | 配置加载                                         |
| `DG_WAVES_V2_V3_simple.js` | 波形源数据                                       |
| `replace_waves.py`         | 波形数据更新脚本                                 |

### 更新波形数据

修改 `DG_WAVES_V2_V3_simple.js` 后运行：

```bash
python3 replace_waves.py
```

## 故障排查

### **看不到 MPRIS 播放器**

- 确认在 Linux 图形会话内运行，`DBUS_SESSION_BUS_ADDRESS` 可用
- 确认已完成 APP 配对（配对后才注册 MPRIS）

### **APP 扫码后无法连接**

- 确认手机与运行程序机器在同一局域网
- 确认监听地址/端口可达（防火墙未阻挡）

### **强度变化不同步**

- 查看 `dglab-mpris.log` 中 WebSocket 收发内容
- 确认 APP 未断开（断开后状态会重置）

### **已知问题 (KDE Plasma)**

在 KDE Plasma 桌面环境下，点击“下一曲/上一曲”切换波形时，由于程序会自动更新 MPRIS 的 `Metadata` (以便您在播放器封面看到当前波形名称)，这会触发 KDE 媒体组件内置的自清洁逻辑，**导致控制中心的进度条被强制归零显示**。虽然代码中已经发射了进度重置信号，但由于异步界面机制通常会被 KDE 自身覆盖。
此为桌面环境 MPRIS 插件自身的响应特性，目前**无法完美解决**。波形和实际强度输出不受影响，当您再次拖动滑块时进度条表现即恢复正常。
