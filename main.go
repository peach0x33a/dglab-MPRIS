package main

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

// ============================================================
// 消息结构体
// ============================================================

// WsMessage 是 WebSocket 通信的统一 JSON 消息格式
type WsMessage struct {
	Type     interface{} `json:"type"`
	ClientID string      `json:"clientId"`
	TargetID string      `json:"targetId"`
	Message  string      `json:"message"`
	Channel  int         `json:"channel,omitempty"`   // 强度通道 (1=A, 2=B)
	Strength int         `json:"strength,omitempty"`  // 强度值
	Time     int         `json:"time,omitempty"`
}

// WsMsgPayload 严格限制与原 Node.js 项目完全一致的四键顺序
// DG-Lab 客户端底层为严格的流式解析，若 type 不是首字段或存在冗余字段会导致整包被丢弃
type WsMsgPayload struct {
	Type     string `json:"type"`
	ClientID string `json:"clientId"`
	TargetID string `json:"targetId"`
	Message  string `json:"message"`
}

// 预设波形数据 (参考自 DG_WAVES_V2_V3_simple.js)
var waveData = []string{
	`["0A0A0A0A00000000", "0A0A0A0A14141414", "0A0A0A0A28282828", "0A0A0A0A3C3C3C3C", "0A0A0A0A50505050", "0A0A0A0A64646464", "0A0A0A0A64646464", "0A0A0A0A64646464", "0A0A0A0A00000000", "0A0A0A0A00000000", "0A0A0A0A00000000", "0A0A0A0A00000000"]`,
	`["0A0A0A0A00000000", "0B0B0B0B10101010", "0D0D0D0D21212121", "0E0E0E0E32323232", "1010101042424242", "1212121253535353", "1313131364646464", "151515155C5C5C5C", "1616161654545454", "181818184C4C4C4C", "1A1A1A1A44444444", "1A1A1A1A00000000", "1B1B1B1B10101010", "1D1D1D1D21212121", "1E1E1E1E32323232", "2020202042424242", "2222222253535353", "2323232364646464", "252525255C5C5C5C", "2626262654545454", "282828284C4C4C4C", "2A2A2A2A44444444", "0A0A0A0A00000000"]`,
	`["0A0A0A0A64646464", "0A0A0A0A00000000", "0A0A0A0A64646464", "0A0A0A0A42424242", "0A0A0A0A21212121", "0A0A0A0A00000000", "0A0A0A0A00000000", "0A0A0A0A00000000", "0A0A0A0A64646464", "0A0A0A0A00000000", "0A0A0A0A64646464", "0A0A0A0A42424242", "0A0A0A0A21212121", "0A0A0A0A00000000", "0A0A0A0A00000000", "0A0A0A0A00000000", "0A0A0A0A64646464", "0A0A0A0A00000000", "0A0A0A0A64646464", "0A0A0A0A42424242", "0A0A0A0A21212121", "0A0A0A0A00000000", "0A0A0A0A00000000", "0A0A0A0A00000000"]`,
	`["0A0A0A0A00000000", "0A0A0A0A64646464", "0A0A0A0A00000000", "0A0A0A0A64646464", "0A0A0A0A00000000", "0A0A0A0A64646464", "0A0A0A0A00000000", "0A0A0A0A64646464", "0A0A0A0A00000000", "0A0A0A0A64646464", "0A0A0A0A00000000", "0A0A0A0A64646464", "0A0A0A0A00000000", "0A0A0A0A64646464", "0A0A0A0A00000000", "0A0A0A0A64646464", "0A0A0A0A00000000", "0A0A0A0A64646464", "0A0A0A0A00000000", "0A0A0A0A64646464", "0A0A0A0A00000000", "0A0A0A0A64646464", "0A0A0A0A00000000", "0A0A0A0A64646464", "0A0A0A0A00000000", "0A0A0A0A64646464", "0A0A0A0A00000000", "0A0A0A0A64646464", "0A0A0A0A00000000", "0A0A0A0A64646464", "0A0A0A0A00000000", "0A0A0A0A64646464", "0A0A0A0A00000000", "0A0A0A0A64646464", "0A0A0A0A00000000", "0A0A0A0A64646464", "0A0A0A0A00000000", "0A0A0A0A64646464", "0A0A0A0A00000000", "0A0A0A0A64646464", "0A0A0A0A00000000", "0A0A0A0A64646464", "0A0A0A0A00000000", "0A0A0A0A64646464", "0A0A0A0A00000000", "0A0A0A0A00000000"]`,
	`["0A0A0A0A00000000", "0A0A0A0A1C1C1C1C", "0A0A0A0A00000000", "0A0A0A0A34343434", "0A0A0A0A00000000", "0A0A0A0A49494949", "0A0A0A0A00000000", "0A0A0A0A57575757", "0A0A0A0A00000000", "0A0A0A0A64646464", "0A0A0A0A00000000", "0A0A0A0A00000000", "0A0A0A0A1C1C1C1C", "0A0A0A0A00000000", "0A0A0A0A34343434", "0A0A0A0A00000000", "0A0A0A0A49494949", "0A0A0A0A00000000", "0A0A0A0A57575757", "0A0A0A0A00000000", "0A0A0A0A64646464", "0A0A0A0A00000000", "0A0A0A0A00000000"]`,
	`["7070707064646464", "7070707064646464", "7070707064646464", "7070707064646464", "7070707064646464", "7070707064646464", "0A0A0A0A00000000", "0A0A0A0A00000000", "0A0A0A0A00000000", "0A0A0A0A00000000", "0A0A0A0A00000000", "0A0A0A0A4B4B4B4B", "0A0A0A0A53535353", "0A0A0A0A5B5B5B5B", "0A0A0A0A64646464", "0A0A0A0A00000000", "0A0A0A0A00000000", "0A0A0A0A00000000", "0A0A0A0A00000000", "0A0A0A0A00000000", "0A0A0A0A00000000", "0A0A0A0A00000000", "0A0A0A0A00000000", "0A0A0A0A00000000", "0A0A0A0A00000000", "0A0A0A0A4B4B4B4B", "0A0A0A0A53535353", "0A0A0A0A5B5B5B5B", "0A0A0A0A64646464", "0A0A0A0A00000000", "0A0A0A0A00000000", "0A0A0A0A00000000", "0A0A0A0A00000000", "0A0A0A0A00000000", "0A0A0A0A00000000"]`,
	`["4A4A4A4A64646464", "4545454564646464", "4040404064646464", "3B3B3B3B64646464", "3636363664646464", "3232323264646464", "2D2D2D2D64646464", "2828282864646464", "2323232364646464", "1E1E1E1E64646464", "1A1A1A1A64646464", "0A0A0A0A64646464", "0A0A0A0A64646464", "0A0A0A0A64646464", "0A0A0A0A64646464", "0A0A0A0A64646464", "0A0A0A0A64646464", "0A0A0A0A64646464", "0A0A0A0A64646464", "0A0A0A0A64646464", "0A0A0A0A64646464"]`,
	`["0A0A0A0A00000000", "0A0A0A0A14141414", "0A0A0A0A28282828", "0A0A0A0A3C3C3C3C", "0A0A0A0A50505050", "0A0A0A0A64646464", "0A0A0A0A00000000", "0A0A0A0A19191919", "0A0A0A0A32323232", "0A0A0A0A4B4B4B4B", "0A0A0A0A64646464", "0A0A0A0A00000000", "0A0A0A0A21212121", "0A0A0A0A42424242", "0A0A0A0A64646464", "0A0A0A0A00000000", "0A0A0A0A32323232", "0A0A0A0A64646464", "0A0A0A0A00000000", "0A0A0A0A64646464", "0A0A0A0A00000000", "0A0A0A0A64646464", "0A0A0A0A00000000", "0A0A0A0A64646464", "0A0A0A0A00000000", "0A0A0A0A64646464", "0A0A0A0A00000000"]`,
	`["0A0A0A0A64646464", "0B0B0B0B64646464", "0D0D0D0D64646464", "0F0F0F0F00000000", "0F0F0F0F64646464", "1111111164646464", "1313131364646464", "1414141400000000", "1414141464646464", "1616161664646464", "1818181864646464", "1A1A1A1A00000000", "1A1A1A1A64646464", "1C1C1C1C64646464", "1D1D1D1D64646464", "1F1F1F1F00000000", "1F1F1F1F64646464", "2121212164646464", "2323232364646464", "2525252500000000", "2525252564646464", "2626262664646464", "2828282864646464", "2A2A2A2A00000000", "2A2A2A2A64646464", "2C2C2C2C64646464", "2E2E2E2E64646464", "3030303000000000"]`,
	`["0A0A0A0A00000000", "0A0A0A0A21212121", "0B0B0B0B42424242", "0C0C0C0C64646464", "0C0C0C0C00000000", "0D0D0D0D21212121", "0E0E0E0E42424242", "0F0F0F0F64646464", "0F0F0F0F00000000", "0F0F0F0F21212121", "1010101042424242", "1111111164646464", "1111111100000000", "1212121221212121", "1313131342424242", "1414141464646464", "1414141400000000", "1414141421212121", "1515151542424242", "1616161664646464", "1616161600000000", "1717171721212121", "1818181842424242", "1919191964646464", "1919191900000000", "1919191921212121", "1A1A1A1A42424242", "1B1B1B1B64646464", "1B1B1B1B00000000", "1C1C1C1C21212121", "1D1D1D1D42424242", "1E1E1E1E64646464", "1E1E1E1E00000000", "1E1E1E1E21212121", "1F1F1F1F42424242", "2020202064646464", "2020202000000000", "2121212121212121", "2222222242424242", "2323232364646464", "2323232300000000", "2323232321212121", "2424242442424242", "2525252564646464", "2525252500000000", "2626262621212121", "2727272742424242", "2828282864646464", "0A0A0A0A00000000", "0A0A0A0A00000000"]`,
	`["0A0A0A0A00000000", "0A0A0A0A32323232", "0A0A0A0A64646464", "0A0A0A0A49494949", "1111111100000000", "1111111132323232", "1111111164646464", "1111111149494949", "1919191900000000", "1919191932323232", "1919191964646464", "1919191949494949", "2121212100000000", "2121212132323232", "2121212164646464", "2121212149494949", "2828282800000000", "2828282832323232", "2828282864646464", "2828282849494949", "3030303000000000", "3030303032323232", "3030303064646464", "3030303049494949", "3838383800000000", "3838383832323232", "3838383864646464", "3838383849494949", "3F3F3F3F00000000", "3F3F3F3F32323232", "3F3F3F3F64646464", "3F3F3F3F49494949", "4747474700000000", "4747474732323232", "4747474764646464", "4747474749494949", "4F4F4F4F00000000", "4F4F4F4F32323232", "4F4F4F4F64646464", "4F4F4F4F49494949", "5656565600000000", "5656565632323232", "5656565664646464", "5656565649494949", "5E5E5E5E00000000", "5E5E5E5E32323232", "5E5E5E5E64646464", "5E5E5E5E49494949", "6464646400000000", "6464646432323232", "6464646464646464", "6464646449494949", "6666666600000000", "6666666632323232", "6666666664646464", "6666666649494949", "0A0A0A0A00000000"]`,
	`["0E0E0E0E21212121", "0E0E0E0E42424242", "0E0E0E0E64646464", "0E0E0E0E21212121", "0E0E0E0E42424242", "0E0E0E0E64646464", "0E0E0E0E21212121", "0E0E0E0E42424242", "0E0E0E0E64646464", "0E0E0E0E21212121", "0E0E0E0E42424242", "0E0E0E0E64646464", "0E0E0E0E21212121", "0E0E0E0E42424242", "0E0E0E0E64646464", "0E0E0E0E21212121", "0E0E0E0E42424242", "0E0E0E0E64646464", "0E0E0E0E21212121", "0E0E0E0E42424242", "0E0E0E0E64646464", "0E0E0E0E21212121", "0E0E0E0E42424242", "0E0E0E0E64646464", "0E0E0E0E21212121", "0E0E0E0E42424242", "0E0E0E0E64646464", "0E0E0E0E21212121", "0E0E0E0E42424242", "0E0E0E0E64646464", "0E0E0E0E21212121", "0E0E0E0E42424242", "0E0E0E0E64646464", "0E0E0E0E21212121", "0E0E0E0E42424242", "0E0E0E0E64646464", "0E0E0E0E21212121", "0E0E0E0E42424242", "0E0E0E0E64646464", "3A3A3A3A64646464", "3A3A3A3A64646464", "3A3A3A3A64646464", "3A3A3A3A64646464", "3A3A3A3A64646464", "3A3A3A3A64646464", "3A3A3A3A64646464", "3A3A3A3A64646464", "3A3A3A3A64646464", "3A3A3A3A64646464", "3A3A3A3A64646464", "3A3A3A3A64646464", "3A3A3A3A64646464", "3A3A3A3A64646464", "3A3A3A3A64646464", "3A3A3A3A64646464", "3A3A3A3A64646464", "3A3A3A3A64646464", "3A3A3A3A64646464", "3A3A3A3A64646464", "3A3A3A3A64646464", "3A3A3A3A64646464", "3A3A3A3A64646464", "3A3A3A3A64646464", "3A3A3A3A64646464", "3A3A3A3A64646464", "3A3A3A3A64646464", "3A3A3A3A64646464", "3A3A3A3A64646464", "3A3A3A3A64646464", "3A3A3A3A64646464", "3A3A3A3A64646464", "3A3A3A3A64646464", "3A3A3A3A64646464", "3A3A3A3A64646464", "3A3A3A3A64646464", "0A0A0A0A00000000", "0A0A0A0A00000000", "0A0A0A0A00000000"]`,
	`["1818181864646464", "1818181864646464", "1818181864646464", "1818181800000000", "1818181800000000", "1818181800000000", "1818181800000000", "1818181864646464", "1818181864646464", "1818181864646464", "1818181800000000", "1818181800000000", "1818181800000000", "1818181800000000", "1818181864646464", "1818181864646464", "1818181864646464", "1818181800000000", "1818181800000000", "1818181800000000", "1818181800000000", "1818181864646464", "1818181864646464", "1818181864646464", "1818181800000000", "1818181800000000", "1818181800000000", "1818181800000000", "1818181864646464", "1818181864646464", "1818181864646464", "1818181800000000", "1818181800000000", "1818181800000000", "1818181800000000", "1818181864646464", "1818181864646464", "1818181864646464", "1818181800000000", "1818181800000000", "1818181800000000", "1818181800000000", "7070707064646464", "7070707064646464", "7070707064646464", "7070707064646464", "7070707064646464", "7070707064646464", "7070707064646464", "7070707064646464", "7070707064646464", "7070707064646464", "7070707064646464", "7070707064646464", "7070707064646464", "7070707064646464", "7070707064646464", "7070707064646464", "7070707064646464", "7070707064646464", "7070707064646464", "7070707064646464", "7070707064646464", "7070707064646464", "7070707064646464", "7070707064646464", "7070707064646464", "7070707064646464", "7070707064646464", "7070707064646464", "7070707064646464", "7070707064646464", "7070707064646464", "7070707064646464", "7070707064646464", "7070707064646464", "7070707064646464", "7070707064646464", "7070707064646464", "7070707064646464", "7070707064646464", "7070707064646464", "0A0A0A0A00000000", "0A0A0A0A00000000"]`,
	`["BEBEBEBE64646464", "BEBEBEBE64646464", "BEBEBEBE64646464", "BEBEBEBE64646464", "BEBEBEBE64646464", "BEBEBEBE64646464", "BEBEBEBE64646464", "BEBEBEBE64646464", "BEBEBEBE64646464", "BEBEBEBE64646464", "BEBEBEBE64646464", "BEBEBEBE64646464", "BEBEBEBE64646464", "BEBEBEBE64646464", "BEBEBEBE64646464", "BEBEBEBE64646464", "BEBEBEBE64646464", "BEBEBEBE64646464", "BEBEBEBE64646464", "BEBEBEBE64646464", "0A0A0A0A00000000", "1010101021212121", "1717171742424242", "1E1E1E1E64646464", "0A0A0A0A00000000", "1010101021212121", "1717171742424242", "1E1E1E1E64646464", "0A0A0A0A00000000", "1010101021212121", "1717171742424242", "1E1E1E1E64646464", "0A0A0A0A00000000", "1010101021212121", "1717171742424242", "1E1E1E1E64646464", "0A0A0A0A00000000", "1010101021212121", "1717171742424242", "1E1E1E1E64646464"]`,
	`["0A0A0A0A00000000", "0C0C0C0C19191919", "0E0E0E0E32323232", "101010104B4B4B4B", "1212121264646464", "1515151564646464", "1717171764646464", "1919191900000000", "1B1B1B1B00000000", "1E1E1E1E00000000", "0A0A0A0A00000000", "0C0C0C0C19191919", "0E0E0E0E32323232", "101010104B4B4B4B", "1212121264646464", "1515151564646464", "1717171764646464", "1919191900000000", "1B1B1B1B00000000", "1E1E1E1E00000000", "0A0A0A0A00000000", "0C0C0C0C19191919", "0E0E0E0E32323232", "101010104B4B4B4B", "1212121264646464", "1515151564646464", "1717171764646464", "1919191900000000", "1B1B1B1B00000000", "1E1E1E1E00000000", "0A0A0A0A00000000", "0C0C0C0C19191919", "0E0E0E0E32323232", "101010104B4B4B4B", "1212121264646464", "1515151564646464", "1717171764646464", "1919191900000000", "1B1B1B1B00000000", "1E1E1E1E00000000", "0A0A0A0A00000000", "0A0A0A0A64646464", "0A0A0A0A00000000", "0A0A0A0A64646464", "0A0A0A0A00000000", "0A0A0A0A64646464", "0A0A0A0A00000000", "0A0A0A0A64646464", "0A0A0A0A00000000", "0A0A0A0A64646464", "0A0A0A0A00000000", "0A0A0A0A64646464", "0A0A0A0A00000000", "0A0A0A0A64646464", "0A0A0A0A00000000", "0A0A0A0A64646464", "0A0A0A0A00000000", "0A0A0A0A64646464", "0A0A0A0A00000000", "0A0A0A0A64646464", "0A0A0A0A00000000", "0A0A0A0A64646464", "0A0A0A0A00000000"]`,
	`["2525252500000000", "222222220B0B0B0B", "2020202016161616", "1E1E1E1E21212121", "1C1C1C1C2C2C2C2C", "1919191937373737", "1717171742424242", "151515154D4D4D4D", "1313131358585858", "1111111164646464", "2525252500000000", "222222220B0B0B0B", "2020202016161616", "1E1E1E1E21212121", "1C1C1C1C2C2C2C2C", "1919191937373737", "1717171742424242", "151515154D4D4D4D", "1313131358585858", "1111111164646464", "2525252500000000", "222222220B0B0B0B", "2020202016161616", "1E1E1E1E21212121", "1C1C1C1C2C2C2C2C", "1919191937373737", "1717171742424242", "151515154D4D4D4D", "1313131358585858", "1111111164646464", "2525252500000000", "222222220B0B0B0B", "2020202016161616", "1E1E1E1E21212121", "1C1C1C1C2C2C2C2C", "1919191937373737", "1717171742424242", "151515154D4D4D4D", "1313131358585858", "1111111164646464", "0A0A0A0A00000000", "0B0B0B0B64646464", "0B0B0B0B00000000", "0C0C0C0C64646464", "0C0C0C0C00000000", "0D0D0D0D64646464", "0D0D0D0D00000000", "0E0E0E0E64646464", "0E0E0E0E00000000", "0F0F0F0F64646464", "0F0F0F0F00000000", "1010101064646464", "1010101000000000", "1111111164646464", "1111111100000000", "1212121264646464", "1212121200000000", "1313131364646464", "1313131300000000", "1414141464646464", "1414141400000000", "1515151564646464", "1515151500000000", "1616161664646464", "1616161600000000", "1717171764646464", "1717171700000000", "1818181864646464", "1818181800000000", "1919191964646464", "1919191900000000", "1A1A1A1A64646464", "1A1A1A1A00000000", "1B1B1B1B64646464", "1B1B1B1B00000000", "1C1C1C1C64646464", "1C1C1C1C00000000", "1D1D1D1D64646464", "1D1D1D1D00000000", "1E1E1E1E64646464", "0A0A0A0A00000000", "0A0A0A0A00000000"]`,
}
var waveNames = []string{"呼吸", "潮汐", "连击", "快速按捏", "按捏渐强", "心跳节奏", "压缩", "节奏步伐", "颗粒摩擦", "渐变弹跳", "波浪涟漪", "雨水冲刷", "变速敲击", "信号灯", "挑逗1", "挑逗2"}

// ============================================================
// 全局状态
// ============================================================

type AppState struct {
	mu         sync.Mutex
	appWriteMu sync.Mutex // 保护 appConn 写操作

	appConn  *websocket.Conn // APP 端的 WebSocket 连接
	clientID   string          // 我们自己的 ID
	targetID   string          // APP 端 ID
	listenAddr string          // 服务监听地址

	strengthA int // A 通道当前强度
	strengthB     int // B 通道当前强度
	maxA          int // A 通道硬上限
	maxB          int // B 通道硬上限
	lastStrengthA int // 上次 A 通道强度 (用于 Play 恢复)
	lastStrengthB int // 上次 B 通道强度

	waveIdxA int // A 通道当前波形索引 (-1=无)
	waveIdxB int // B 通道当前波形索引 (-1=无)

	paired     bool // 是否已配对
	mprisReady bool // MPRIS 是否已注册

	propsA    *prop.Properties // A 通道 MPRIS 属性
	propsB    *prop.Properties // B 通道 MPRIS 属性
	dbusConnA *dbus.Conn
	dbusConnB *dbus.Conn
}

var (
	state    = &AppState{}
	upgrader = websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			// 仅允许同源或空 origin（移动端 APP 直接连接无 browser context）
			origin := r.Header.Get("Origin")
			if origin == "" {
				return true
			}
			// 允许同源请求
			if origin == "null" || strings.HasPrefix(origin, "file://") {
				return true
			}
			return false
		},
	}
	boundCh           = make(chan struct{}, 1)
	quitCh            = make(chan struct{}) // 用于优雅停止后台 goroutine
	qrDisconnectedCh  = make(chan struct{}) // 通知 TUI 显示二维码
)

// ============================================================
// 工具函数
// ============================================================

// generateID 生成随机十六进制 ID
func generateID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		log.Fatalf("[安全] 生成随机 ID 失败: %v", err)
	}
	return hex.EncodeToString(b)
}

type ifaceAddr struct {
	Name string
	IP   string
}

// getLocalAddresses 获取所有可用的网络接口及其 IPv4 地址
func getLocalAddresses() []ifaceAddr {
	var result []ifaceAddr
	ifaces, err := net.Interfaces()
	if err != nil {
		return result
	}
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			if ipNet, ok := addr.(*net.IPNet); ok && ipNet.IP.To4() != nil {
				result = append(result, ifaceAddr{Name: iface.Name, IP: ipNet.IP.String()})
			}
		}
	}
	return result
}

// selectBindAddress 交互式选择绑定地址
func selectBindAddress(reader *bufio.Reader) string {
	addrs := getLocalAddresses()
	if len(addrs) == 0 {
		fmt.Println("  ⚠️ 未检测到可用网络接口，使用 0.0.0.0")
		return "0.0.0.0"
	}

	fmt.Println("  可用网络接口：")
	fmt.Println()
	for i, a := range addrs {
		fmt.Printf("    [%d] %-12s %s\n", i+1, a.Name, a.IP)
	}
	fmt.Printf("    [%d] %-12s %s\n", len(addrs)+1, "all", "0.0.0.0 (所有接口)")
	fmt.Printf("    [%d] %-12s %s\n", len(addrs)+2, "custom", "手动输入地址")
	fmt.Println()

	for {
		fmt.Printf("  请选择绑定地址 [1-%d]: ", len(addrs)+2)
		input, err := reader.ReadString('\n')
		if err != nil {
			log.Fatalf("读取输入失败: %v\n", err)
		}
		input = strings.TrimSpace(input)
		choice, err := strconv.Atoi(input)
		if err != nil || choice < 1 || choice > len(addrs)+2 {
			fmt.Println("  ❌ 无效选择，请重新输入！")
			continue
		}

		if choice <= len(addrs) {
			selected := addrs[choice-1]
			fmt.Printf("  ✅ 已选择: %s (%s)\n\n", selected.IP, selected.Name)
			return selected.IP
		} else if choice == len(addrs)+1 {
			fmt.Println("  ✅ 已选择: 0.0.0.0 (所有接口)")
			fmt.Println("  ⚠️  QR 码将使用第一个可用的局域网 IP")
			fmt.Println()
			return "0.0.0.0"
		} else {
			fmt.Print("  请输入自定义地址 (如 192.168.1.100): ")
			custom, err := reader.ReadString('\n')
			if err != nil {
				log.Fatalf("读取输入失败: %v\n", err)
			}
			custom = strings.TrimSpace(custom)
			if custom == "" {
				fmt.Println("  ❌ 地址不能为空！")
				continue
			}
			fmt.Printf("  ✅ 已选择: %s\n\n", custom)
			return custom
		}
	}
}

// resetTerminalReset 尝试重置终端状态，兼容 Konsole 等终端
func resetTerminal() {
	// 尝试 tput reset（优先）
	cmd := exec.Command("tput", "reset")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		// fallback 到 reset 命令
		cmd2 := exec.Command("reset")
		cmd2.Stdout = os.Stdout
		cmd2.Stderr = os.Stderr
		cmd2.Run()
	}
}

// selectAddressOnlyTUI 仅用于地址选择，返回选中的地址
func selectAddressOnlyTUI(addrs []ifaceAddr, port int, clientID string) string {
	quitCh := make(chan string)
	model := NewSelectAddressModel(addrs, quitCh, port, clientID)

	p := tea.NewProgram(model)

	doneCh := make(chan struct{})
	var result string

	go func() {
		defer close(doneCh)
		if _, err := p.Run(); err != nil {
			log.Printf("启动 TUI 失败: %v\n", err)
		}
		resetTerminal()
	}()

	select {
	case addr := <-quitCh:
		result = addr
	case <-doneCh:
		result = ""
	}

	return result
}

// selectBindAddressTUI 使用 TUI 交互式选择绑定地址并等待 APP 连接
// 返回 true 如果用户选择退出，false 如果继续
func selectBindAddressTUI(bindAddr string, port int, clientID string) bool {
	model := NewSelectAddressModel(getLocalAddresses(), nil, port, clientID)

	p := tea.NewProgram(model)
	if _, err := p.Run(); err != nil {
		log.Printf("启动 TUI 失败: %v\n", err)
	}
	resetTerminal()
	return model.shouldQuit
}

// writeToApp 线程安全地向 APP 发送消息
func writeToApp(data []byte) error {
	state.appWriteMu.Lock()
	defer state.appWriteMu.Unlock()
	if state.appConn == nil {
		return fmt.Errorf("no connection")
	}
	return state.appConn.WriteMessage(websocket.TextMessage, data)
}

// ============================================================
// MPRIS 接口实现
// ============================================================

// MprisRoot 处理 org.mpris.MediaPlayer2
type MprisRoot struct {
	channelId int
}

func (r *MprisRoot) Quit() *dbus.Error {
	log.Printf("[MPRIS] Quit 被调用 (Channel %d)...\n", r.channelId)

	state.mu.Lock()
	defer state.mu.Unlock()
	if state.appConn == nil || state.clientID == "" || state.targetID == "" {
		return nil
	}
	sendStrengthLocked(2, r.channelId, 0)
	return nil
}

func (r *MprisRoot) Raise() *dbus.Error { return nil }

// MprisPlayer 处理 org.mpris.MediaPlayer2.Player
type MprisPlayer struct {
	channelId int
}

func (m *MprisPlayer) getStrengthAndMax() (int, int, int) {
	if m.channelId == 1 {
		return state.strengthA, state.lastStrengthA, state.maxA
	}
	return state.strengthB, state.lastStrengthB, state.maxB
}

func (m *MprisPlayer) Play() *dbus.Error {
	state.mu.Lock()
	defer state.mu.Unlock()
	if !state.paired {
		return nil
	}

	cur, last, maxLimit := m.getStrengthAndMax()
	if cur > 0 {
		return nil
	}

	target := last
	if target <= 0 {
		target = 0
	}
	if target > maxLimit {
		target = maxLimit
	}
	sendStrengthLocked(2, m.channelId, target)
	updatePlaybackStatusLocked(m.channelId, "Playing")
	return nil
}

func (m *MprisPlayer) Pause() *dbus.Error {
	state.mu.Lock()
	defer state.mu.Unlock()
	if !state.paired {
		return nil
	}

	cur, _, _ := m.getStrengthAndMax()
	if cur > 0 {
		if m.channelId == 1 {
			state.lastStrengthA = cur
		} else {
			state.lastStrengthB = cur
		}
	}
	sendStrengthLocked(2, m.channelId, 0)
	updatePlaybackStatusLocked(m.channelId, "Paused")
	return nil
}

func (m *MprisPlayer) PlayPause() *dbus.Error {
	state.mu.Lock()
	cur, _, _ := m.getStrengthAndMax()
	isPlaying := cur > 0
	state.mu.Unlock()

	if isPlaying {
		return m.Pause()
	}
	return m.Play()
}

func (m *MprisPlayer) Stop() *dbus.Error {
	state.mu.Lock()
	defer state.mu.Unlock()
	if !state.paired {
		return nil
	}
	sendStrengthLocked(2, m.channelId, 0)
	updatePlaybackStatusLocked(m.channelId, "Stopped")
	return nil
}

func setWaveIndex(channelId int, offset int) {
	state.mu.Lock()
	defer state.mu.Unlock()
	idx := -1
	if channelId == 1 {
		idx = state.waveIdxA
	} else {
		idx = state.waveIdxB
	}

	idx += offset
	if idx >= len(waveData) {
		idx = -1
	} else if idx < -1 {
		idx = len(waveData) - 1
	}

	if channelId == 1 {
		state.waveIdxA = idx
	} else {
		state.waveIdxB = idx
	}

	updateMetadataLocked()
}

func (m *MprisPlayer) Next() *dbus.Error {
	if !state.paired {
		return nil
	}
	setWaveIndex(m.channelId, 1)
	return nil
}

func (m *MprisPlayer) Previous() *dbus.Error {
	if !state.paired {
		return nil
	}
	setWaveIndex(m.channelId, -1)
	return nil
}

// microsecondsPerUnit 表示微秒与强度单位之间的转换因子
// MPRIS 使用微秒作为时间单位，而本应用中 1 单位强度 = 1,000,000 微秒 = 1 秒
const microsecondsPerUnit = 1000000

// Seek 偏移当前位置（微秒 → 强度值）
// 根据 MPRIS2 规范，offset 是相对于指定位置的时间偏移量（微秒）
// 正值表示快进，负值表示快退
func (m *MprisPlayer) Seek(offset int64) *dbus.Error {
	state.mu.Lock()
	defer state.mu.Unlock()

	// 未配对时直接返回，符合 MPRIS 规范的幂等性要求
	if !state.paired {
		return nil
	}

	// 确保 MPRIS 服务已就绪
	if !state.mprisReady {
		return nil
	}

	currentStrength, _, maxLimit := m.getStrengthAndMax()

	// 计算新位置：当前强度(微秒) + 偏移量(微秒)
	currentPos := int64(currentStrength) * microsecondsPerUnit
	newPos := currentPos + offset

	// 转换为强度值
	newStrength := int(newPos / microsecondsPerUnit)

	// 钳制到有效范围 [0, maxLimit]
	if newStrength < 0 {
		newStrength = 0
	}
	if newStrength > maxLimit {
		newStrength = maxLimit
	}

	// 无实际位置变更时跳过更新，避免不必要的通信
	if newStrength == currentStrength {
		return nil
	}

	log.Printf("[MPRIS] Seek(%d) → pos:%d str:%d (Channel %d)", offset, newPos, newStrength, m.channelId)

	sendStrengthLocked(2, m.channelId, newStrength)

	// 发射 Seeked 信号以通知外部客户端（如 KDE Plasma）位置已变更
	// 仅对当前通道的 D-Bus 连接发射信号
	newPosMicroseconds := int64(newStrength) * microsecondsPerUnit
	if m.channelId == 1 {
		emitPositionChanged(state.dbusConnA, newPosMicroseconds)
	} else {
		emitPositionChanged(state.dbusConnB, newPosMicroseconds)
	}

	return nil
}

// SetPosition 设置绝对位置（微秒 → 强度值）
func (m *MprisPlayer) SetPosition(trackID dbus.ObjectPath, position int64) *dbus.Error {
	state.mu.Lock()
	defer state.mu.Unlock()
	if !state.paired {
		return nil
	}

	log.Printf("[MPRIS] SetPosition(%s, %d) called by KDE for Channel %d", trackID, position, m.channelId)

	_, _, maxLimit := m.getStrengthAndMax()
	// microseconds -> seconds
	newStrength := int(position / 1000000)
	if newStrength < 0 {
		newStrength = 0
	}
	if newStrength > maxLimit {
		newStrength = maxLimit
	}
	sendStrengthLocked(2, m.channelId, newStrength)
	return nil
}

// OpenUri 不支持
func (m *MprisPlayer) OpenUri(uri string) *dbus.Error { return nil }

// ============================================================
// WebSocket 通信
// ============================================================

// sendStrength 发送强度控制消息（需在未锁定时调用）
func sendStrength(msgType int, channel int, strength int) {
	state.mu.Lock()
	defer state.mu.Unlock()
	sendStrengthLocked(msgType, channel, strength)
}

// sendStrengthLocked 发送强度控制消息（调用者已持有 state.mu）
func sendStrengthLocked(msgType int, channel int, strength int) {
	if state.appConn == nil || state.clientID == "" || state.targetID == "" {
		return
	}

	// 1:1 还原 Node.js 版本的包体格式与强顺序
	msg := WsMsgPayload{
		Type:     "msg",
		ClientID: state.clientID, // 发送方 UUID
		TargetID: state.targetID, // 接收方 UUID
		Message:  fmt.Sprintf("strength-%d+%d+%d", channel, msgType, strength),
	}

	data, err := json.Marshal(msg)
	if err != nil {
		log.Printf("[WS] 序列化消息失败: %v\n", err)
		return
	}

	log.Printf("[WS] 正在下发指令至硬件 APP: %s", string(data))

	if err := writeToApp(data); err != nil {
		log.Printf("[WS] 发送消息失败: %v\n", err)
	}
}

// sendClear 发送清空波形指令
func sendClear(channel string) {
	state.mu.Lock()
	defer state.mu.Unlock()
	if state.appConn == nil || state.clientID == "" || state.targetID == "" {
		return
	}

	chNum := "1"
	if channel == "B" {
		chNum = "2"
	}
	
	msg := WsMsgPayload{
		Type:     "msg",
		ClientID: state.clientID,
		TargetID: state.targetID,
		Message:  "clear-" + chNum,
	}
	data, _ := json.Marshal(msg)
	log.Printf("[WS] 正在下发指令至硬件 APP: %s", string(data))
	writeToApp(data)
}

// sendPulse 发送波形控制消息（需在未锁定时调用）
func sendPulse(channelName string, waveDataStr string) {
	state.mu.Lock()
	defer state.mu.Unlock()
	if state.appConn == nil || state.clientID == "" || state.targetID == "" {
		return
	}

	msg := WsMsgPayload{
		Type:     "msg",
		ClientID: state.clientID,
		TargetID: state.targetID,
		Message:  fmt.Sprintf("pulse-%s:%s", channelName, waveDataStr),
	}

	data, err := json.Marshal(msg)
	if err != nil {
		log.Printf("[WS] 序列化波形消息失败: %v\n", err)
		return
	}

	log.Printf("[WS] 正在下发指令至硬件 APP: %s", string(data))

	if err := writeToApp(data); err != nil {
		log.Printf("[WS] 发送波形消息失败: %v\n", err)
	}
}

// ============================================================
// WebSocket 服务端
// ============================================================

// wsHandler 处理来自 APP 的 WebSocket 连接
func wsHandler(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("[WS] 升级连接失败: %v\n", err)
		return
	}

	state.mu.Lock()
	if state.paired {
		state.mu.Unlock()
		log.Println("[WS] 已有 APP 连接，拒绝新连接")
		conn.Close()
		return
	}
	state.mu.Unlock()

	// 为 APP 分配 ID
	appID := generateID()
	log.Printf("[WS] 新连接来自 %s，分配 APP ID: %s\n", r.RemoteAddr, appID)

	// 发送 bind 消息，给客户发送自己的 ID 以绑定 (1:1 Node.js 逻辑)
	bindMsg := WsMessage{
		Type:     "bind",
		ClientID: state.clientID, // 前端 (Web) 的 ID
		TargetID: appID,          // 刚刚自动分配的 APP 的 ID
		Message:  "targetId",
	}
	data, _ := json.Marshal(bindMsg)
	if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
		log.Printf("[WS] 发送 bind 消息失败: %v\n", err)
		conn.Close()
		return
	}

	// 连接断开时的清理
	defer func() {
		conn.Close()
		state.mu.Lock()
		if state.appConn == conn {
			state.paired = false
			state.appConn = nil
			state.targetID = ""
		}
		state.mu.Unlock()
		log.Println("[WS] ⚠️ APP 连接已断开")
		// 通知 TUI 显示二维码（使用 select 防止通道关闭后 panic）
		select {
		case qrDisconnectedCh <- struct{}{}:
		default:
		}
	}()

	// 消息循环
	for {
		_, rawMsg, err := conn.ReadMessage()
		if err != nil {
			log.Printf("[WS] 读取消息失败: %v\n", err)
			return
		}

		var msg WsMessage
		if err := json.Unmarshal(rawMsg, &msg); err != nil {
			log.Printf("[WS] 解析消息失败: %s\n", string(rawMsg))
			continue
		}

		msgType := fmt.Sprintf("%v", msg.Type)

		switch msgType {
		case "bind":
			state.mu.Lock()
			// APP 可能将 TargetID 设置为我们在 QR 码中的 state.clientID，或者我们在上方刚分配并下发的 appID
			if msg.TargetID == state.clientID || msg.TargetID == appID {
				// 获取 APP 的真实 ID (它在 clientId 字段中发给我们)
				appRealID := msg.ClientID
				if appRealID == "" {
					appRealID = appID
				}

				state.targetID = appRealID
				state.appConn = conn
				state.paired = true
				state.mu.Unlock()

				// 发送绑定确认 (200)
				confirmMsg := WsMessage{
					Type:     "bind",
					ClientID: state.clientID,
					TargetID: state.targetID,
					Message:  "200",
				}
				d, _ := json.Marshal(confirmMsg)
				writeToApp(d)

				log.Printf("[WS] ✅ 配对成功！APP ID: %s\n", state.targetID)
				// TUI will handle displaying this message
				// fmt.Println("\n╔═══════════════════════════════════════════╗")
				// fmt.Println("║          ✅ APP 已成功连接！              ║")
				// fmt.Println("╚═══════════════════════════════════════════╝")

				// 启动心跳
				go startHeartbeat(conn, appID, quitCh)

				// 通知主 goroutine
				select {
				case boundCh <- struct{}{}:
				default:
				}
			} else {
				state.mu.Unlock()
				log.Printf("[WS] 绑定目标不匹配: got=%s, want=%s\n", msg.TargetID, state.clientID)
			}

		case "msg":
			handleAppMsg(msg)

		case "heartbeat":
			log.Println("[WS] 💓 心跳 (来自 APP)")

		case "break":
			log.Printf("[WS] ⚠️ APP 主动断开 (code: %s)\n", msg.Message)
			return

		default:
			if strings.Contains(msg.Message, "strength") || strings.Contains(msg.Message, "feedback") {
				handleAppMsg(msg)
			} else {
				log.Printf("[WS] 未知消息类型: %s\n", msgType)
			}
		}
	}
}

// startHeartbeat 定期向 APP 发送心跳（支持通过 quitCh 优雅停止）
func startHeartbeat(conn *websocket.Conn, appID string, quitCh <-chan struct{}) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-quitCh:
			log.Println("[WS] 心跳已停止")
			return
		case <-ticker.C:
			state.mu.Lock()
			if state.appConn != conn {
				state.mu.Unlock()
				return
			}
			state.mu.Unlock()

			hb := WsMessage{
				Type:     "heartbeat",
				ClientID: state.clientID,
				TargetID: state.targetID,
				Message:  "heartbeat",
			}
			data, _ := json.Marshal(hb)
			if err := writeToApp(data); err != nil {
				log.Printf("[WS] 发送心跳失败: %v\n", err)
				return
			}
		}
	}
}

// waveLoop 波形循环 goroutine（统一实现，支持优雅停止）
func waveLoop(channelName string, getWaveIdx func() int, quitCh <-chan struct{}) {
	lastIdx := -1
	for {
		select {
		case <-quitCh:
			log.Printf("[WS] 波形循环 %s 已停止\n", channelName)
			return
		default:
		}

		state.mu.Lock()
		idx := getWaveIdx()
		state.mu.Unlock()

		if idx != lastIdx {
			sendClear(channelName)
			lastIdx = idx
			state.mu.Lock()
			updateMetadataLocked()
			state.mu.Unlock()
			time.Sleep(150 * time.Millisecond) // 发送 clear 后稍作延迟，防止指令冲撞
		}

		if idx >= 0 && idx < len(waveData) {
			waveStr := waveData[idx]
			sendPulse(channelName, waveStr)

			elements := strings.Count(waveStr, ",") + 1
			duration := elements * 100
			sleepTime := duration - 150
			if sleepTime < 100 {
				sleepTime = 100
			}

			// 拆分 sleep，以便能更快响应波形切换
			for i := 0; i < sleepTime; i += 100 {
				select {
				case <-quitCh:
					return
				case <-time.After(100 * time.Millisecond):
				}
				state.mu.Lock()
				newIdx := getWaveIdx()
				state.mu.Unlock()
				if newIdx != idx {
					break
				}
			}
		} else {
			time.Sleep(200 * time.Millisecond)
		}
	}
}

var strengthRegex = regexp.MustCompile(`strength-(\d+)\+(\d+)\+(\d+)\+(\d+)`)

func handleAppMsg(msg WsMessage) {
	if strings.Contains(msg.Message, "strength") {
		matches := strengthRegex.FindStringSubmatch(msg.Message)
		if len(matches) < 5 {
			// 尝试另一种格式
			re2 := regexp.MustCompile(`\d+`)
			nums := re2.FindAllString(msg.Message, -1)
			if len(nums) >= 4 {
				matches = []string{"", nums[0], nums[1], nums[2], nums[3]}
			}
		}
		if len(matches) >= 5 {
			a, _ := strconv.Atoi(matches[1])
			b, _ := strconv.Atoi(matches[2])
			aMax, _ := strconv.Atoi(matches[3])
			bMax, _ := strconv.Atoi(matches[4])

			state.mu.Lock()
			needMetaUpdate := (aMax != state.maxA) || (bMax != state.maxB)
			state.strengthA = a
			state.strengthB = b
			state.maxA = aMax
			state.maxB = bMax

			// 更新 MPRIS 属性
			if state.mprisReady {
				// “设置上限后手动设置一次进度”
				// 注意：在 KDE Plasma 中，一旦发出 Metadata 改变事件 (如波形名改变或长度改变)，
				// KDE 会认为换歌了，并且将进度强制归零！
				// 因此我们必须先将 Position 属性的值更新到正确值，再触发 Metadata 更新！
				// 这样当 KDE 在 Metadata 改变后立即查询 Position 时，能读到正确值而非 0。
				var posA, posB int64
				if state.propsA != nil {
					posA = int64(a) * 1000000
					state.propsA.SetMust("org.mpris.MediaPlayer2.Player", "Position", dbus.MakeVariant(posA))
				}
				if state.propsB != nil {
					posB = int64(b) * 1000000
					state.propsB.SetMust("org.mpris.MediaPlayer2.Player", "Position", dbus.MakeVariant(posB))
				}
				if needMetaUpdate {
					updateMetadataLocked()
				}
				// Metadata 更新完成后，发射 Seeked 信号通知 KDE 位置已变更
				if state.propsA != nil {
					emitPositionChanged(state.dbusConnA, posA)
				}
				if state.propsB != nil {
					emitPositionChanged(state.dbusConnB, posB)
				}
			}
			state.mu.Unlock()
		}
	}
}

// ============================================================
// MPRIS 辅助函数
// ============================================================

func updatePlaybackStatusLocked(channelId int, status string) {
	if channelId == 1 && state.propsA != nil {
		state.propsA.SetMust("org.mpris.MediaPlayer2.Player", "PlaybackStatus", dbus.MakeVariant(status))
	} else if channelId == 2 && state.propsB != nil {
		state.propsB.SetMust("org.mpris.MediaPlayer2.Player", "PlaybackStatus", dbus.MakeVariant(status))
	}
}

func emitPositionChanged(conn *dbus.Conn, position int64) {
	if conn == nil {
		return
	}
	// MPRIS 规范要求 Position 变动时不发 PropertiesChanged (因为它太频繁了)
	// 如果希望客户端(如KDE Plasma)感知外部导致的跳跃，需派发 Seeked 信号
	conn.Emit(dbus.ObjectPath("/org/mpris/MediaPlayer2"), "org.mpris.MediaPlayer2.Player.Seeked", position)
}

func updateMetadataLocked() {
	if state.propsA != nil {
		albumA := fmt.Sprintf("硬上限: %d", state.maxA)
		if state.waveIdxA >= 0 {
			albumA += fmt.Sprintf(" | 波形: %s", waveNames[state.waveIdxA])
		} else {
			albumA += " | 波形: 手动"
		}
		metaA := map[string]dbus.Variant{
			"mpris:trackid": dbus.MakeVariant(dbus.ObjectPath("/org/mpris/MediaPlayer2/Track/1")),
			"mpris:length":  dbus.MakeVariant(int64(state.maxA) * 1000000),
			"xesam:title":   dbus.MakeVariant("A 通道 (DG-LAB)"),
			"xesam:artist":  dbus.MakeVariant([]string{"DG-LAB 控制器"}),
			"xesam:album":   dbus.MakeVariant(albumA),
		}
		state.propsA.SetMust("org.mpris.MediaPlayer2.Player", "Metadata", dbus.MakeVariant(metaA))
	}

	if state.propsB != nil {
		albumB := fmt.Sprintf("硬上限: %d", state.maxB)
		if state.waveIdxB >= 0 {
			albumB += fmt.Sprintf(" | 波形: %s", waveNames[state.waveIdxB])
		} else {
			albumB += " | 波形: 手动"
		}
		metaB := map[string]dbus.Variant{
			"mpris:trackid": dbus.MakeVariant(dbus.ObjectPath("/org/mpris/MediaPlayer2/Track/1")),
			"mpris:length":  dbus.MakeVariant(int64(state.maxB) * 1000000),
			"xesam:title":   dbus.MakeVariant("B 通道 (DG-LAB)"),
			"xesam:artist":  dbus.MakeVariant([]string{"DG-LAB 控制器"}),
			"xesam:album":   dbus.MakeVariant(albumB),
		}
		state.propsB.SetMust("org.mpris.MediaPlayer2.Player", "Metadata", dbus.MakeVariant(metaB))
	}
}

// registerMPRISChannel 注册一个通道的 MPRIS D-Bus 服务
func registerMPRISChannel(channelName string, channelId int, identity string) (*prop.Properties, *dbus.Conn, error) {
	conn, err := dbus.ConnectSessionBus()
	if err != nil {
		return nil, nil, fmt.Errorf("连接 D-Bus 失败: %w", err)
	}

	root := &MprisRoot{channelId: channelId}
	player := &MprisPlayer{channelId: channelId}

	conn.Export(root, "/org/mpris/MediaPlayer2", "org.mpris.MediaPlayer2")
	conn.Export(player, "/org/mpris/MediaPlayer2", "org.mpris.MediaPlayer2.Player")

	state.mu.Lock()
	maxLimit := state.maxA
	if channelId == 2 {
		maxLimit = state.maxB
	}
	state.mu.Unlock()

	metadata := map[string]dbus.Variant{
		"mpris:trackid": dbus.MakeVariant(dbus.ObjectPath("/org/mpris/MediaPlayer2/Track/1")),
		"mpris:length":  dbus.MakeVariant(int64(maxLimit) * 1000000),
		"xesam:title":   dbus.MakeVariant(identity + " 已连接"),
		"xesam:artist":  dbus.MakeVariant([]string{"DG-LAB 控制器"}),
		"xesam:album":   dbus.MakeVariant("等待强度数据..."),
	}

	propsSpec := map[string]map[string]*prop.Prop{
		"org.mpris.MediaPlayer2": {
			"Identity":            {Value: identity, Writable: false, Emit: prop.EmitTrue},
			"CanQuit":             {Value: true, Writable: false, Emit: prop.EmitTrue},
			"CanRaise":            {Value: false, Writable: false, Emit: prop.EmitTrue},
			"HasTrackList":        {Value: false, Writable: false, Emit: prop.EmitTrue},
			"DesktopEntry":        {Value: "org.kde.konsole", Writable: false, Emit: prop.EmitTrue},
			"SupportedUriSchemes": {Value: []string{}, Writable: false, Emit: prop.EmitTrue},
			"SupportedMimeTypes":  {Value: []string{}, Writable: false, Emit: prop.EmitTrue},
		},
		"org.mpris.MediaPlayer2.Player": {
			"PlaybackStatus": {Value: "Playing", Writable: false, Emit: prop.EmitTrue},
			"LoopStatus":     {Value: "None", Writable: false, Emit: prop.EmitTrue},
			"Rate":           {Value: 0.0, Writable: false, Emit: prop.EmitTrue},
			"MinimumRate":    {Value: 0.0, Writable: false, Emit: prop.EmitTrue},
			"MaximumRate":    {Value: 0.0, Writable: false, Emit: prop.EmitTrue},
			"Shuffle":        {Value: false, Writable: false, Emit: prop.EmitTrue},
			"Metadata":       {Value: metadata, Writable: false, Emit: prop.EmitTrue},
			"Volume":         {Value: 1.0, Writable: false, Emit: prop.EmitTrue},
			"Position":       {Value: int64(0), Writable: false, Emit: prop.EmitFalse},
			"CanControl":     {Value: true, Writable: false, Emit: prop.EmitTrue},
			"CanPlay":        {Value: true, Writable: false, Emit: prop.EmitTrue},
			"CanPause":       {Value: true, Writable: false, Emit: prop.EmitTrue},
			"CanSeek":        {Value: true, Writable: false, Emit: prop.EmitTrue},
			"CanGoNext":      {Value: true, Writable: false, Emit: prop.EmitTrue},
			"CanGoPrevious":  {Value: true, Writable: false, Emit: prop.EmitTrue},
		},
	}

	props, err := prop.Export(conn, "/org/mpris/MediaPlayer2", propsSpec)
	if err != nil {
		return nil, nil, fmt.Errorf("导出 MPRIS 属性失败: %w", err)
	}

	busName := fmt.Sprintf("org.mpris.MediaPlayer2.DGLab_%s", channelName)
	reply, err := conn.RequestName(busName, dbus.NameFlagReplaceExisting)
	if err != nil {
		return nil, nil, fmt.Errorf("请求 D-Bus 名称失败: %w", err)
	}
	if reply != dbus.RequestNameReplyPrimaryOwner {
		return nil, nil, fmt.Errorf("未能获取 D-Bus 名称所有权: %s", busName)
	}

	log.Printf("[MPRIS] ✅ MPRIS 服务已注册: %s\n", busName)
	return props, conn, nil
}

// ============================================================
// 主程序
// ============================================================

func main() {
	f, _ := os.OpenFile("dglab-mpris.log", os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0666)
	if f != nil {
		log.SetOutput(f)
	} else {
		log.SetOutput(io.Discard)
	}

	// 加载配置文件（如果存在）
	cfg, err := LoadConfig()
	if err != nil {
		log.Printf("[配置] 加载配置失败: %v，使用默认配置\n", err)
	}

	port := flag.Int("port", 9999, "WebSocket 服务监听端口")
	host := flag.String("host", "", "QR 码中使用的主机地址 (默认自动检测局域网 IP)")
	flag.Parse()

	// 命令行参数优先于配置文件
	if cfg != nil {
		if *host == "" && cfg.Host != "" {
			*host = cfg.Host
		}
		if *port == 9999 && cfg.Port != 9999 {
			*port = cfg.Port
		}
	}

	// 生成本机 clientId
	state.clientID = generateID()
	state.waveIdxA = -1
	state.waveIdxB = -1
	state.maxA = 200 // 默认上限，连上后会自动更新
	state.maxB = 200 // 默认上限，连上后会自动更新

	// 启动后台波形流式循环（支持通过 quitCh 优雅停止）
	go waveLoop("A", func() int { return state.waveIdxA }, quitCh)
	go waveLoop("B", func() int { return state.waveIdxB }, quitCh)

	// 确定绑定地址
	bindAddr := *host
	if bindAddr == "" {
		addrs := getLocalAddresses()
		if len(addrs) == 0 {
			fmt.Println("  ⚠️ 未检测到可用网络接口，使用 0.0.0.0")
			bindAddr = "0.0.0.0"
		} else {
			// 先用简单 TUI 选择地址
			bindAddr = selectAddressOnlyTUI(addrs, *port, state.clientID)
			if bindAddr == "" {
				log.Println("未选择地址，退出")
				os.Exit(0)
			}
		}
	}

	// 计算 qrHost（无论地址是来自参数还是选择）
	addrs := getLocalAddresses()
	qrHost := bindAddr
	if bindAddr == "0.0.0.0" {
		for _, a := range addrs {
			if a.IP != "127.0.0.1" {
				qrHost = a.IP
				break
			}
		}
		if qrHost == "0.0.0.0" || qrHost == "" {
			qrHost = "localhost"
		}
	}
	// 使用完整 TUI 显示二维码并等待连接
	if selectBindAddressTUI(bindAddr, *port, state.clientID) {
		log.Println("用户退出 TUI，退出程序")
		os.Exit(0)
	}

	// TUI 已退出，设置监听地址并继续主程序
	listenAddr := fmt.Sprintf("%s:%d", bindAddr, *port)
	state.listenAddr = listenAddr
	wsAddrForDisplay := fmt.Sprintf("ws://%s:%d/", qrHost, *port)

	// 重置终端状态（清除 TUI 残留的屏幕状态）
	fmt.Print("\033[2J\033[H")
	fmt.Printf("  监听地址: %s\n", listenAddr)
	fmt.Printf("  WS 地址:  %s\n\n", wsAddrForDisplay)

	// 生成二维码
	qrContent := fmt.Sprintf("https://www.dungeon-lab.com/app-download.php#DGLAB-SOCKET#%s%s", wsAddrForDisplay, state.clientID)

	fmt.Println("  等待 APP 扫码连接...")

	// 启动 WebSocket 服务
	http.HandleFunc("/", wsHandler)
	go func() {
		log.Printf("[WS] 正在监听 %s ...\n", listenAddr)
		if err := http.ListenAndServe(listenAddr, nil); err != nil {
			log.Fatalf("[WS] 启动服务失败: %v\n", err)
		}
	}()

	// 等待配对完成
	<-boundCh
	fmt.Println()



	// 注册 MPRIS A 通道
	propsA, connA, err := registerMPRISChannel("A", 1, "DG-LAB A通道")
	if err != nil {
		log.Fatalf("[MPRIS] A 通道注册失败: %v\n", err)
	}
	// 注册 MPRIS B 通道
	propsB, connB, err := registerMPRISChannel("B", 2, "DG-LAB B通道")
	if err != nil {
		log.Fatalf("[MPRIS] B 通道注册失败: %v\n", err)
	}

	state.mu.Lock()
	state.propsA = propsA
	state.propsB = propsB
	state.dbusConnA = connA
	state.dbusConnB = connB
	state.mprisReady = true
	state.mu.Unlock()

	// 启动 TUI 界面
	p := tea.NewProgram(NewAppModel(qrContent, qrDisconnectedCh))
	if _, err := p.Run(); err != nil {
		fmt.Printf("启动 UI 失败: %v\n", err)
		os.Exit(1)
	}

	log.Println("\n正在退出...")
	// 通知后台 goroutine 优雅停止
	close(quitCh)

	state.mu.Lock()
	if state.appConn != nil {
		// 安全归零
		sendStrengthLocked(2, 1, 0)
		sendStrengthLocked(2, 2, 0)
		state.appConn.Close()
	}
	if state.dbusConnA != nil {
		state.dbusConnA.Close()
	}
	if state.dbusConnB != nil {
		state.dbusConnB.Close()
	}
	state.mu.Unlock()

	os.Exit(0)
}
