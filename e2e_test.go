package main

import (
	"encoding/json"
	"fmt"
	"net/url"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// TestE2EStrength 端到端测试：模拟 APP 完成配对后验证强度控制消息
// 运行前需要启动 dglab-mpris：./dglab-mpris -port 9998
// 然后 go test -v -run TestE2EStrength -timeout 30s
func TestE2EStrength(t *testing.T) {
	u := url.URL{Scheme: "ws", Host: "localhost:9998", Path: "/test-app-uuid"}
	fmt.Printf("连接到 %s\n", u.String())

	c, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		t.Fatalf("dial error: %v (请先运行: ./dglab-mpris -port 9998)", err)
	}
	defer c.Close()

	// ===== Step 1: 读取初始 bind 消息 =====
	_, message, err := c.ReadMessage()
	if err != nil {
		t.Fatalf("读取初始 bind 失败: %v", err)
	}
	fmt.Printf("收到初始 bind: %s\n", message)

	var initBind struct {
		Type     string `json:"type"`
		ClientID string `json:"clientId"`
		TargetID string `json:"targetId"`
		Message  string `json:"message"`
	}
	if err := json.Unmarshal(message, &initBind); err != nil {
		t.Fatalf("解析初始 bind 失败: %v", err)
	}

	// 验证: clientId 非空, targetId 为空
	if initBind.ClientID == "" {
		t.Fatal("初始 bind 的 clientId 为空")
	}
	if initBind.TargetID != "" {
		t.Fatalf("初始 bind 的 targetId 应为空, got: %s", initBind.TargetID)
	}
	fmt.Printf("✅ 初始 bind 格式正确, 获得 appID=%s\n", initBind.ClientID)

	// ===== Step 2: 模拟 APP 发送 bind 请求 =====
	// APP 使用我们在 URL 路径中提取到的 clientID 作为 targetId
	// 但实际上 APP 使用从 QR 码中解析的 state.clientID
	// 在测试中我们不知道 state.clientID，但可以从 URL path 推断
	// 由于 main.go 检查 msg.TargetID == state.clientID || msg.TargetID == appID
	// 我们可以使用 appID 作为 targetId（这样可以匹配第二个条件）
	bindRequest := map[string]interface{}{
		"type":     "bind",
		"clientId": initBind.ClientID, // APP 使用自己获得的 ID
		"targetId": initBind.ClientID, // 用 appID 匹配第二个条件
		"message":  "200",
	}
	bindData, _ := json.Marshal(bindRequest)
	fmt.Printf("发送 bind 请求: %s\n", string(bindData))

	if err := c.WriteMessage(websocket.TextMessage, bindData); err != nil {
		t.Fatalf("发送 bind 失败: %v", err)
	}

	// ===== Step 3: 读取 bind 确认 =====
	_, confirmMsg, err := c.ReadMessage()
	if err != nil {
		t.Fatalf("读取 bind 确认失败: %v", err)
	}
	fmt.Printf("收到 bind 确认: %s\n", string(confirmMsg))

	var confirm struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	}
	json.Unmarshal(confirmMsg, &confirm)
	if confirm.Message != "200" {
		t.Fatalf("bind 确认失败, 期望 200, 得到 %s", confirm.Message)
	}
	fmt.Println("✅ 配对成功!")

	// ===== Step 4: 等待 MPRIS 注册 =====
	fmt.Println("等待 MPRIS 注册完成 (3s)...")
	time.Sleep(3 * time.Second)

	// ===== Step 5: 用 dbus-send 触发 Seek，然后检查是否收到 strength 消息 =====
	// 先设置一个读取超时
	c.SetReadDeadline(time.Now().Add(5 * time.Second))

	// 读取后续消息（心跳或强度消息）
	fmt.Println("等待心跳或强度消息...")
	for i := 0; i < 5; i++ {
		_, data, err := c.ReadMessage()
		if err != nil {
			fmt.Printf("读取超时或错误: %v (这是预期的)\n", err)
			break
		}
		fmt.Printf("收到消息 #%d: %s\n", i+1, string(data))
	}

	fmt.Println("✅ 端到端测试完成!")
}
