package ssh

import (
	"bufio"
	"io"
	"log"
	"strings"
	"sync"
	"time"

	"qinjiagateway/demo"
	"qinjiagateway/model"
)

// demoSessionState 演示会话的底层状态（不暴露给外部）
type demoSessionState struct {
	stdin  *io.PipeWriter
	stdout *io.PipeReader
	shell  *demo.SimShell
	mu     sync.Mutex
	closed bool
}

// connectDemo 创建演示模式的 SSH 会话（模拟 PTY）
func connectDemo(sessID string, userID uint, server *model.Server, cols, rows int) (*Session, error) {
	hostname := strings.TrimPrefix(server.IP, "demo://")
	shell := demo.NewSimShell(hostname)

	stdinR, stdinW := io.Pipe()
	stdoutR, stdoutW := io.Pipe()

	state := &demoSessionState{
		stdin:  stdinW,
		stdout: stdoutR,
		shell:  shell,
	}

	// 后台 goroutine：读取命令、执行、写入输出
	go func() {
		defer stdoutW.Close()
		defer stdinR.Close()

		// 发送欢迎信息和初始提示符
		stdoutW.Write([]byte("Welcome to Ubuntu 22.04.3 LTS (GNU/Linux 5.15.0-91-generic x86_64)\r\n\r\n"))
		stdoutW.Write([]byte(" * Documentation:  https://help.ubuntu.com\r\n"))
		stdoutW.Write([]byte(" * Management:     https://landscape.canonical.com\r\n"))
		stdoutW.Write([]byte(" * Support:        https://ubuntu.com/advantage\r\n\r\n"))
		stdoutW.Write([]byte(shell.Prompt()))

		// 按 rune 读取：中文等 UTF-8 多字节输入收齐后再整体回显，避免终端乱码
		br := bufio.NewReader(stdinR)
		var cmdLine strings.Builder

		for {
			r, _, err := br.ReadRune()
			if err != nil {
				break
			}

			switch {
			// Ctrl+C：放弃当前输入行，回显 ^C 与新提示符（不回显原始控制字符）
			case r == 0x03:
				cmdLine.Reset()
				stdoutW.Write([]byte("^C\r\n" + shell.Prompt()))

			// 回车：执行命令，输出结果与新提示符
			case r == '\r' || r == '\n':
				cmd := cmdLine.String()
				cmdLine.Reset()
				output := shell.Exec(cmd)
				if output != "" {
					stdoutW.Write([]byte("\r\n" + output))
				} else {
					stdoutW.Write([]byte("\r\n"))
				}
				stdoutW.Write([]byte(shell.Prompt()))

			// 退格：从行尾删一个 rune，按显示宽度回显 \b \b 真正擦除
			case r == 0x7f || r == 0x08:
				s := cmdLine.String()
				if len(s) > 0 {
					rs := []rune(s)
					last := rs[len(rs)-1]
					cmdLine.Reset()
					cmdLine.WriteString(string(rs[:len(rs)-1]))
					w := 1
					if (last >= 0x2E80 && last <= 0x9FFF) || (last >= 0xFF00 && last <= 0xFFEF) || (last >= 0x3000 && last <= 0x30FF) {
						w = 2
					}
					back := strings.Repeat("\b", w)
					stdoutW.Write([]byte(back + strings.Repeat(" ", w) + back))
				}

			// 可打印字符（含中文）：收齐 rune 后回显并入行
			case r >= 32:
				cmdLine.WriteRune(r)
				stdoutW.Write([]byte(string(r)))

			// 其余控制字符（方向键转义、Tab 等）不回显
			default:
				// ESC 转义序列（方向键/Home/End 等）：[ 或 O 开头，读到终止符（@~）整体丢弃，
				// 防止 [A 之类的字面量被当作可打印字符污染输入行
				if r == 0x1b {
					if next, _, err := br.ReadRune(); err == nil && (next == '[' || next == 'O') {
						for {
							fin, _, err := br.ReadRune()
							if err != nil || (fin >= '@' && fin <= '~') {
								break
							}
						}
					}
				}
			}
		}
	}()

	s := &Session{
		ID:         sessID,
		UserID:     userID,
		ServerID:   server.ID,
		ServerName: server.Name,
		CreatedAt:  time.Now(),
		demo:       state,
	}

	mu.Lock()
	sessions[sessID] = s
	mu.Unlock()

	log.Printf("演示模式：创建模拟 SSH 会话 %s -> %s", sessID, server.Name)
	return s, nil
}

// isDemo 判断会话是否为演示模式
func (s *Session) isDemo() bool {
	return s.demo != nil
}
