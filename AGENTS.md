# AGENTS.md — dglab-MPRIS

Bridge DG-LAB devices to Linux MPRIS/D-Bus media control ecosystem.

## Build & Run Commands

```bash
# Install dependencies
go mod tidy

# Run directly (development)
go run .

# Build binary
make
./bin/dglab-mpris

# Run with make
make run

# Clean build artifacts
make clean

# Reinstall dependencies
make deps
```

### Single Test Execution

This project has no test files (`go test` will return "no test files"). If tests are added later:

```bash
# Run all tests
go test ./...

# Run tests for a specific package
go test .

# Run tests with verbose output
go test -v .

# Run tests matching a pattern
go test -run "TestName" .
```

## Code Style Guidelines

### General

- **Language**: Go 1.25+
- **Package**: Single `package main` (all code in main binary)
- **Formatter**: Standard `go fmt` (enforced by Makefile/build)
- **Linter**: No custom linter config; use `golangci-lint` if added later

### Project Structure

```
main.go    — WebSocket server, pairing, MPRIS implementation, heartbeat
tui.go     — Bubble Tea terminal UI
config.go  — Configuration loading from config.yaml
```

### Imports

Standard library imports first, then third-party, separated by blank line:

```go
import (
    "bufio"
    "crypto/rand"
    "encoding/hex"
    "encoding/json"
    "flag"
    "fmt"
    "io"
    "log"
    "net"
    "net/http"
    "os"
    "os/exec"
    "regexp"
    "strconv"
    "strings"
    "sync"
    "time"

    tea "github.com/charmbracelet/bubbletea"

    "github.com/godbus/dbus/v5"
    "github.com/godbus/dbus/v5/prop"
    "github.com/gorilla/websocket"
)
```

### Naming Conventions

| Element | Convention | Example |
|---------|-----------|---------|
| Variables | camelCase | `appConn`, `strengthA` |
| Constants | camelCase or CamelCase | `microsecondsPerUnit` |
| Functions | PascalCase | `sendStrength`, `handleAppMsg` |
| Types | PascalCase | `AppState`, `WsMessage` |
| Struct fields | camelCase | `clientID`, `targetID` |
| Packages | lowercase | (single package `main`) |
| JSON fields | camelCase | `json:"clientId"` |
| YAML fields | lowercase | `yaml:"host"` |

### Type Annotations

- Use explicit struct tags for JSON/YAML serialization
- Pointer types only when necessary (`*websocket.Conn` when nullable)
- Interface types for abstractions (`tea.Model`, `tea.Cmd`)

### Error Handling

```go
// Preferred patterns (from codebase):
if err != nil {
    return nil, nil, fmt.Errorf("连接 D-Bus 失败: %w", err)
}

// Log and continue for non-fatal errors:
if err != nil {
    log.Printf("[WS] 序列化消息失败: %v\n", err)
    return
}

// Fatal errors:
if err != nil {
    log.Fatalf("[安全] 生成随机 ID 失败: %v", err)
}
```

### Mutex & Concurrency

- Global state protected by `sync.Mutex` (`state.mu`)
- Write operations use `Lock()`/`Unlock()` pattern
- Use `defer` for unlock in functions with early returns:
```go
state.mu.Lock()
defer state.mu.Unlock()
```

### Comments

- Chinese comments for clarity (project language is Chinese)
- Section dividers for code organization:
```go
// ============================================================
// WebSocket 通信
// ============================================================
```

### Logging

- Use `log.Printf` for errors and diagnostics
- Use `log.Fatalf` only for fatal startup errors
- Log prefix format: `[WS]`, `[MPRIS]`, `[配置]` etc.

### Configuration

- CLI flags via `flag` package
- YAML config via `gopkg.in/yaml.v3`
- CLI args override config file values

### TUI (Bubble Tea)

- Models implement `tea.Model` interface
- Use `tea.Msg` types for internal messages
- Commands via `tea.Cmd` return values
- Use `lipgloss` for styled output

### D-Bus / MPRIS

- Use `godbus/dbus/v5` with `prop` for property exposure
- Export objects at `/org/mpris/MediaPlayer2`
- Bus names: `org.mpris.MediaPlayer2.DGLab_A`, `org.mpris.MediaPlayer2.DGLab_B`
- Emit `Seeked` signal after position changes

### WebSocket

- Use `gorilla/websocket`
- JSON message format with `WsMessage` struct
- Custom protocol: `Message` field contains sub-commands like `strength-1+2+50`

### Key Files

| File | Purpose |
|------|---------|
| `main.go` | Entry point, WebSocket server, MPRIS D-Bus service, heartbeat |
| `tui.go` | Bubble Tea TUI for address selection and status display |
| `config.go` | YAML configuration loading |

### Dependencies

| Package | Purpose |
|---------|---------|
| `charmbracelet/bubbletea` | TUI framework |
| `charmbracelet/lipgloss` | Terminal styling |
| `godbus/dbus/v5` | D-Bus & MPRIS integration |
| `gorilla/websocket` | WebSocket server/client |
| `mdp/qrterminal/v3` | QR code terminal output |
| `skip2/go-qrcode` | QR code PNG generation |
| `gopkg.in/yaml.v3` | YAML config parsing |

## Development Notes

- Binary output: `./bin/dglab-mpris`
- Log file: `dglab-mpris.log` (created in working directory)
- Config file: `config.yaml` (same directory as binary)
- QR code images saved as `qr_<host>:<port>.png`
