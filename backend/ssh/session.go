package ssh

import (
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
	"time"

	"qinjiagateway/model"
	"qinjiagateway/utils"

	"golang.org/x/crypto/ssh"
)

// Session SSH终端会话
type Session struct {
	ID         string
	UserID     uint
	ServerID   uint
	ServerName string
	Conn       *ssh.Client
	Session    *ssh.Session
	Stdin      io.WriteCloser
	Stdout     io.Reader
	CreatedAt  time.Time
	mu         sync.Mutex
	closed     bool
	demo       *demoSessionState // 演示模式非 nil
}

var (
	sessions = make(map[string]*Session)
	mu       sync.RWMutex
)

// Connect 建立SSH连接并创建PTY会话
func Connect(sessID string, userID uint, server *model.Server, cols, rows int) (*Session, error) {
	// 演示模式：模拟服务器走内置命令模拟器
	if strings.HasPrefix(server.IP, "demo://") {
		return connectDemo(sessID, userID, server, cols, rows)
	}

	// 解密敏感字段；解密失败说明加密密钥已变更，禁止拿密文尝试认证（必败且误导）
	if plain, ok := utils.DecryptEx(server.Password); ok {
		server.Password = plain
	} else {
		return nil, fmt.Errorf("服务器密码无法解密（加密密钥已变更），请在服务器列表中重新编辑并保存该服务器密码")
	}
	if plain, ok := utils.DecryptEx(server.PrivateKey); ok {
		server.PrivateKey = plain
	} else {
		return nil, fmt.Errorf("服务器私钥无法解密（加密密钥已变更），请在服务器列表中重新编辑并保存该服务器私钥")
	}
	var authMethods []ssh.AuthMethod
	if server.AuthType == "key" {
		signer, err := ssh.ParsePrivateKey([]byte(server.PrivateKey))
		if err != nil {
			return nil, fmt.Errorf("解析私钥失败: %v", err)
		}
		authMethods = append(authMethods, ssh.PublicKeys(signer))
	} else {
		authMethods = append(authMethods, ssh.Password(server.Password))
		// 键盘交互认证（服务器禁用直接密码认证时的替代方式）
		authMethods = append(authMethods, ssh.KeyboardInteractive(func(name, instruction string, questions []string, echos []bool) ([]string, error) {
			answers := make([]string, len(questions))
			for i := range questions {
				answers[i] = server.Password
			}
			return answers, nil
		}))
	}

	config := &ssh.ClientConfig{
		User:            server.Username,
		Auth:            authMethods,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         10 * time.Second,
	}

	addr := fmt.Sprintf("%s:%d", server.IP, server.Port)
	conn, err := ssh.Dial("tcp", addr, config)
	if err != nil {
		return nil, fmt.Errorf("连接服务器失败: %s", friendlySSHError(err))
	}

	session, err := conn.NewSession()
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("创建会话失败: %v", err)
	}

	// 请求PTY终端
	modes := ssh.TerminalModes{
		ssh.ECHO:          1,
		ssh.TTY_OP_ISPEED: 14400,
		ssh.TTY_OP_OSPEED: 14400,
	}
	if err := session.RequestPty("xterm", rows, cols, modes); err != nil {
		session.Close()
		conn.Close()
		return nil, fmt.Errorf("请求PTY失败: %v", err)
	}

	stdin, err := session.StdinPipe()
	if err != nil {
		session.Close()
		conn.Close()
		return nil, err
	}
	stdout, err := session.StdoutPipe()
	if err != nil {
		session.Close()
		conn.Close()
		return nil, err
	}
	session.Stderr = session.Stdout

	if err := session.Shell(); err != nil {
		session.Close()
		conn.Close()
		return nil, fmt.Errorf("启动Shell失败: %v", err)
	}

	s := &Session{
		ID:         sessID,
		UserID:     userID,
		ServerID:   server.ID,
		ServerName: server.Name,
		Conn:       conn,
		Session:    session,
		Stdin:      stdin,
		Stdout:     stdout,
		CreatedAt:  time.Now(),
	}

	mu.Lock()
	sessions[sessID] = s
	mu.Unlock()

	return s, nil
}

// GetSession 获取会话
func GetSession(id string) *Session {
	mu.RLock()
	defer mu.RUnlock()
	return sessions[id]
}

// RemoveSession 移除并关闭会话
func RemoveSession(id string) {
	mu.Lock()
	if s, ok := sessions[id]; ok {
		s.Close()
		delete(sessions, id)
	}
	mu.Unlock()
}

// GetAllSessions 获取所有在线会话
func GetAllSessions() []map[string]interface{} {
	mu.RLock()
	defer mu.RUnlock()
	var list []map[string]interface{}
	for id, s := range sessions {
		list = append(list, map[string]interface{}{
			"session_id":  id,
			"user_id":     s.UserID,
			"server_id":   s.ServerID,
			"server_name": s.ServerName,
			"created_at":  s.CreatedAt,
		})
	}
	return list
}

// Close 关闭会话
func (s *Session) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return
	}
	s.closed = true
	if s.isDemo() {
		s.demo.mu.Lock()
		s.demo.closed = true
		s.demo.stdin.Close()
		s.demo.stdout.Close()
		s.demo.mu.Unlock()
		return
	}
	if s.Session != nil {
		s.Session.Close()
	}
	if s.Conn != nil {
		s.Conn.Close()
	}
}

// Resize 调整终端大小
func (s *Session) Resize(cols, rows int) error {
	if s.isDemo() {
		return nil // 演示模式忽略 resize
	}
	return s.Session.WindowChange(rows, cols)
}

// Write 写入数据到终端
func (s *Session) Write(data []byte) error {
	if s.isDemo() {
		_, err := s.demo.stdin.Write(data)
		return err
	}
	_, err := s.Stdin.Write(data)
	return err
}

// Read 读取终端输出
func (s *Session) Read(p []byte) (int, error) {
	if s.isDemo() {
		return s.demo.stdout.Read(p)
	}
	return s.Stdout.Read(p)
}

// IsClosed 是否已关闭
func (s *Session) IsClosed() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.isDemo() {
		s.demo.mu.Lock()
		defer s.demo.mu.Unlock()
		return s.demo.closed
	}
	return s.closed
}

// Ping 检测连接是否存活
func (s *Session) Ping() error {
	if s.isDemo() {
		if s.IsClosed() {
			return io.ErrClosedPipe
		}
		return nil
	}
	_, _, err := s.Conn.SendRequest("ping", true, nil)
	if err != nil {
		return err
	}
	return nil
}

// 确保net包被引用（用于可能的网络诊断）
var _ = net.Dial

// friendlySSHError 将SSH错误转换为用户可理解的提示
func friendlySSHError(err error) string {
	msg := err.Error()
	switch {
	case strings.Contains(msg, "unable to authenticate"):
		return fmt.Sprintf("%s —— 认证被拒绝。请检查：① 平台中配置的用户名/密码是否正确；"+
			"② root账号需在目标服务器 /etc/ssh/sshd_config 中开启 PermitRootLogin yes 和 PasswordAuthentication yes（修改后执行 systemctl restart sshd）；"+
			"③ 若服务器仅允许密钥登录，请把认证方式改为密钥并导入私钥", msg)
	case strings.Contains(msg, "no common algorithm"):
		return fmt.Sprintf("%s —— 与目标服务器无共同加密算法（服务器SSH版本过旧或过新），需调整算法配置", msg)
	case strings.Contains(msg, "connection refused"):
		return "目标服务器拒绝连接，请检查SSH端口是否正确、sshd服务是否运行"
	case strings.Contains(msg, "i/o timeout") || strings.Contains(msg, "timeout"):
		return "连接超时，请检查IP是否可达、防火墙/安全组是否放行SSH端口"
	default:
		return msg
	}
}
