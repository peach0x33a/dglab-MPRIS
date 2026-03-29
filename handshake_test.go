package main

import (
	"encoding/json"
	"fmt"
	"net/url"
	"testing"

	"github.com/gorilla/websocket"
)

// TestHandshake 验证 WebSocket 握手是否符合 dglab-socketv2 规范
func TestHandshake(t *testing.T) {
	u := url.URL{Scheme: "ws", Host: "localhost:9999", Path: "/test-uuid"}
	fmt.Printf("Connecting to %s\n", u.String())

	c, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		t.Fatalf("dial error: %v (请确保程序已在 9999 端口运行)", err)
	}
	defer c.Close()

	// 1. 读取第一个消息
	_, message, err := c.ReadMessage()
	if err != nil {
		t.Fatalf("read message error: %v", err)
	}
	fmt.Printf("Received: %s\n", message)

	var msg struct {
		Type     string `json:"type"`
		ClientID string `json:"clientId"`
		TargetID string `json:"targetId"`
		Message  string `json:"message"`
	}
	if err := json.Unmarshal(message, &msg); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	// 2. 验证格式对齐 dglab-socketv2
	if msg.Type != "bind" {
		t.Errorf("Expected type 'bind', got '%s'", msg.Type)
	}
	if msg.ClientID == "" {
		t.Error("Expected non-empty clientId")
	}
	if msg.TargetID != "" {
		t.Errorf("Expected empty targetId (dglab-socketv2 style), got '%s'", msg.TargetID)
	}

	fmt.Println("✅ Handshake logic verified!")
}
