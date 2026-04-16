package redispub

import (
	"bufio"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"
)

// ChannelPlayerOffline Redis channel 名稱：玩家下線事件（Gate 發佈，各服務訂閱）
const ChannelPlayerOffline = "events/player_offline"

// DefaultDialTimeout 預設連線超時
var DefaultDialTimeout = 500 * time.Millisecond

// DefaultIOTimeout 預設讀寫超時
var DefaultIOTimeout = 500 * time.Millisecond

// Publish 向 Redis 發佈一條訊息到指定 channel（一次一連線，適合低頻發佈如斷線事件）。
func Publish(addr, channel, message string) error {
	if addr == "" {
		return fmt.Errorf("redis addr is empty")
	}
	cmd := buildPublishCmd(channel, message)
	conn, err := net.DialTimeout("tcp", addr, DefaultDialTimeout)
	if err != nil {
		return fmt.Errorf("dial redis %s: %w", addr, err)
	}
	defer conn.Close()

	conn.SetDeadline(time.Now().Add(DefaultIOTimeout))
	if _, err := conn.Write([]byte(cmd)); err != nil {
		return fmt.Errorf("write redis: %w", err)
	}

	br := bufio.NewReader(conn)
	line, err := br.ReadString('\n')
	if err != nil {
		return fmt.Errorf("read redis: %w", err)
	}
	line = strings.TrimSpace(line)
	if strings.HasPrefix(line, "-") {
		return fmt.Errorf("redis error: %s", line)
	}
	return nil
}

func buildPublishCmd(channel, message string) string {
	// RESP: *3\r\n$7\r\nPUBLISH\r\n$N\r\n{channel}\r\n$M\r\n{message}\r\n
	var b strings.Builder
	b.WriteString("*3\r\n")
	b.WriteString("$7\r\nPUBLISH\r\n")
	b.WriteString("$")
	b.WriteString(strconv.Itoa(len(channel)))
	b.WriteString("\r\n")
	b.WriteString(channel)
	b.WriteString("\r\n")
	b.WriteString("$")
	b.WriteString(strconv.Itoa(len(message)))
	b.WriteString("\r\n")
	b.WriteString(message)
	b.WriteString("\r\n")
	return b.String()
}
