package handler

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"qinjiagateway/config"
	"qinjiagateway/dao"
	"qinjiagateway/demo"
	"qinjiagateway/model"
	"qinjiagateway/utils"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"golang.org/x/crypto/ssh"
)

// ===== H5 远程桌面（RDP via guacamole 桥接） =====
//
// 浏览器(guacamole-common-js) ⇄ WebSocket ⇄ 本桥 ⇄ TCP ⇄ guacd ⇄ RDP
// 本桥负责与 guacd 完成握手（select/args/size/image/ready），
// ready 之后进入纯双向转发；协议为文本指令（len.data[,|;]）。

type rdpTokenInfo struct {
	ServerID uint
	UserID   uint
	IP       string
	Port     int
	Proto    string // rdp / vnc
	Username string
	Password string
	ExpireAt time.Time
}

// 一次性连接凭证（60 秒内有效，用后即焚）
var rdpTokens sync.Map // token -> rdpTokenInfo

var errGuacProto = errors.New("guacamole 协议错误")

// guacReadInstruction 从 guacd 读取一条完整指令（含结尾 ';'）。
// 协议：元素为 <长度>.<数据>，元素间以 ',' 分隔，指令以 ';' 结束。
// 长度按 Unicode 字符数（与官方客户端一致，guacd 按 UTF-8 感知解析）。
func guacReadInstruction(r *bufio.Reader) (string, error) {
	var sb strings.Builder
	for {
		// 读长度（数字）
		length := 0
		hasDigit := false
		for {
			b, err := r.ReadByte()
			if err != nil {
				return "", err
			}
			if b >= '0' && b <= '9' {
				length = length*10 + int(b-'0')
				hasDigit = true
				continue
			}
			if b == '.' && hasDigit {
				break
			}
			return "", errGuacProto
		}
		// 按字符数读数据：UTF-8 起始字节（及非法字节）计 1 个字符，
		// 续接字节(10xxxxxx)不计，避免把多字节字符截断
		data := make([]byte, 0, length*4)
		for count := 0; count < length; {
			b, err := r.ReadByte()
			if err != nil {
				return "", err
			}
			data = append(data, b)
			if b&0xC0 != 0x80 {
				count++
			}
		}
		sb.Write(data)
		// 读分隔符
		sep, err := r.ReadByte()
		if err != nil {
			return "", err
		}
		if sep == ';' {
			return sb.String(), nil
		}
		if sep != ',' {
			return "", errGuacProto
		}
		sb.WriteByte(',')
	}
}

// guacEncodeInstruction 按协议编码一条指令
func guacEncodeInstruction(args ...string) string {
	var sb strings.Builder
	for i, a := range args {
		sb.WriteString(strconv.Itoa(utf8.RuneCountInString(a)))
		sb.WriteByte('.')
		sb.WriteString(a)
		if i == len(args)-1 {
			sb.WriteByte(';')
		} else {
			sb.WriteByte(',')
		}
	}
	return sb.String()
}

// RDPToken 为指定服务器签发一次性 H5 远程桌面连接凭证
func RDPToken(c *gin.Context) {
	if demo.IsDemo() {
		utils.Fail(c, "演示模式不支持远程桌面")
		return
	}
	idStr := c.Query("server_id")
	id64, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil || id64 == 0 {
		utils.Fail(c, "参数错误")
		return
	}
	userID, _ := c.Get("user_id")
	role, _ := c.Get("role")

	// 与 TerminalConnect 相同的权限校验：普通用户仅限被授权的服务器
	var server model.Server
	if err := dao.DB.First(&server, id64).Error; err != nil {
		utils.Fail(c, "服务器不存在")
		return
	}
	if role != "admin" {
		var cnt int64
		dao.DB.Model(&model.UserServer{}).Where("user_id = ? AND server_id = ?", userID, id64).Count(&cnt)
		if cnt == 0 {
			utils.Fail(c, "无该服务器访问权限")
			return
		}
	}
	// guacd 未配置（空或 "-"）时由内置 RDP 引擎（rdpd）接管，无需拦截

	// 协议与端口：Windows 走 RDP；Linux 走 VNC（需在编辑中开启 VNC 端口）
	proto := "rdp"
	port := server.Port
	if server.OsType != "windows" {
		proto = "vnc"
		if server.VncPort <= 0 {
			utils.Fail(c, "该服务器未开启远程桌面：请在编辑中配置 VNC 端口（如 5901）")
			return
		}
		port = server.VncPort
	}
	// RDP（NLA）与 VNC（vncpasswd）均使用服务器密码字段；
	// VNC 密码为空时仍可连接（目标机未设 vncpasswd 时走 None 认证）
	var plain string
	if p, ok := utils.DecryptEx(server.Password); ok {
		plain = p
	}
	if proto == "rdp" && plain == "" {
		utils.Fail(c, "该服务器未配置密码，无法使用远程桌面（RDP 需密码认证）")
		return
	}

	token := uuid.New().String()
	rdpTokens.Store(token, rdpTokenInfo{
		ServerID: server.ID,
		UserID:   userID.(uint),
		IP:       server.IP,
		Port:     port,
		Proto:    proto,
		Username: server.Username,
		Password: plain,
		ExpireAt: time.Now().Add(60 * time.Second),
	})
	utils.Success(c, gin.H{"token": token, "proto": proto})
}

// RDPSocket H5 远程桌面 WebSocket：/ws/rdp?token=&w=&h=
func RDPSocket(c *gin.Context) {
	// guacamole WebSocketTunnel 建连时会无条件追加 "?<data>"（前端 client.connect()
	// 未传 data 时产生 "?undefined" 垃圾尾巴，如 "&h=1080?undefined"），
	// 截掉第二个 "?" 及之后内容再解析，避免尺寸参数解析失败回落默认值 768。
	rawQuery := c.Request.URL.RawQuery
	if i := strings.Index(rawQuery, "?"); i >= 0 {
		rawQuery = rawQuery[:i]
	}
	q, _ := url.ParseQuery(rawQuery)
	token := q.Get("token")
	// 前端编码能力协商：WebCodecs 可用时请求 h264 视频通道（rdpd 据此决定
	// 是否启用 ffmpeg 帧间压缩管线；外置 guacd 不识别该参数，无影响）
	codec := q.Get("codec")
	// 画质档位（fluent/normal/high → CRF 25/21/17）与渲染帧率上限（30/60），
	// 随握手 args 下发给内置 rdpd；外置 guacd 的参数名列表不含它们，自动忽略
	quality := q.Get("quality")
	fps, _ := strconv.Atoi(q.Get("fps"))
	w, h := 1920, 1080
	if v, err := strconv.Atoi(q.Get("w")); err == nil && v > 0 {
		w = v
	}
	if v, err := strconv.Atoi(q.Get("h")); err == nil && v > 0 {
		h = v
	}
	if w > 3840 {
		w = 3840
	}
	if h > 2160 {
		h = 2160
	}

	// 先升级 WebSocket：后续校验失败时通过 guac error 指令下发真实原因，
	// 避免前端把所有失败统一显示为「无法建立 WebSocket 通道」
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Println("RDP WebSocket升级失败:", err)
		return
	}
	defer conn.Close()

	// 取一次性凭证
	v, ok := rdpTokens.LoadAndDelete(token)
	if !ok {
		guacFail(conn, "连接凭证无效或已过期，请从服务器管理页重新打开远程桌面")
		return
	}
	info := v.(rdpTokenInfo)
	if time.Now().After(info.ExpireAt) {
		guacFail(conn, "连接凭证已过期，请从服务器管理页重新打开远程桌面")
		return
	}
	guacdAddr := config.Global.Server.GuacdAddr
	if guacdAddr == "" || guacdAddr == "-" {
		// 未配置 guacd：使用内置 RDP 引擎（rdpd）
		log.Printf("RDP 内置引擎：server=%d 前端请求 %dx%d codec=%s quality=%s fps=%d", info.ServerID, w, h, codec, quality, fps)
		builtinRDPSession(conn, info, w, h, codec, quality, fps)
		return
	}

	// 连接 guacd
	guacConn, err := net.DialTimeout("tcp", guacdAddr, 5*time.Second)
	if err != nil {
		guacSendError(conn, "无法连接 guacd 服务器（"+guacdAddr+"）：请确认 guacamole-server 已启动")
		return
	}
	defer guacConn.Close()
	guacReader := bufio.NewReaderSize(guacConn, 32*1024)
	guacWriter := bufio.NewWriter(guacConn)

	// 握手：select -> 收 args -> 发 size/image/audio/video + args 值 -> 收 ready
	if err := guacHandshake(guacReader, guacWriter, guacWriter, info, w, h, codec, quality, fps); err != nil {
		guacSendError(conn, "远程桌面连接失败："+err.Error())
		return
	}

	log.Printf("H5 远程桌面已连接：server=%d user=%d %s:%d", info.ServerID, info.UserID, info.IP, info.Port)

	// ===== 握手完成，进入纯双向转发 =====
	done := make(chan struct{}, 2)

	// guacd -> 浏览器（按指令边界重组后整条发送，原因同 rdp_builtin.go）
	go func() {
		for {
			line, err := guacReader.ReadBytes(';')
			if len(line) > 0 {
				if werr := conn.WriteMessage(websocket.TextMessage, line); werr != nil {
					break
				}
			}
			if err != nil {
				break
			}
		}
		done <- struct{}{}
	}()

	// 浏览器 -> guacd（原样转发，guacd 与前端解析器均可处理分片）
	go func() {
		for {
			_, data, err := conn.ReadMessage()
			if err != nil {
				break
			}
			if _, werr := guacConn.Write(data); werr != nil {
				break
			}
		}
		done <- struct{}{}
	}()

	<-done
	log.Printf("H5 远程桌面已断开：server=%d user=%d", info.ServerID, info.UserID)
}

// guacHandshake 与 guacd（或内置 rdpd）完成连接握手。
// w 为底层写出器；flusher 非 nil 时在关键节点刷缓冲（TCP 场景）。
// codec 为前端请求的视频编码通道（h264 = WebCodecs，随 args 下发；
// quality/fps 为画质档位与渲染帧率。外置 guacd 的 args 列表不含这三个
// 参数名，不会下发，向后兼容）。
func guacHandshake(r *bufio.Reader, w io.Writer, flusher interface{ Flush() error }, info rdpTokenInfo, width, height int, codec, quality string, fps int) error {
	writeInstr := func(s string) error {
		_, err := io.WriteString(w, s)
		return err
	}
	flush := func() error {
		if flusher != nil {
			return flusher.Flush()
		}
		return nil
	}
	proto := info.Proto
	if proto == "" {
		proto = "rdp"
	}
	// 1) select 协议（rdp / vnc）
	if err := writeInstr(guacEncodeInstruction("select", proto)); err != nil {
		return err
	}
	if err := flush(); err != nil {
		return err
	}
	// 2) 收 args（参数名列表）
	instr, err := guacReadInstruction(r)
	if err != nil {
		return err
	}
	parts := strings.Split(instr, ",")
	if len(parts) == 0 || parts[0] != "args" {
		return errGuacProto
	}
	names := parts[1:]

	// 3) 按参数名回填值（未知参数留空，guacd 忽略空值）。
	// VNC 仅用 hostname/port/password（username/security 置空，rdpd 与外置 guacd 均忽略）
	values := map[string]string{
		"hostname":      info.IP,
		"port":          strconv.Itoa(info.Port),
		"password":      info.Password,
		"username":      "",
		"security":      "",
		"ignore-cert":   "true",
		"resize-method": "",
		"client-name":   "QinJiaGateway",
		"codec":         codec,
		"quality":       quality,
		"fps":           strconv.Itoa(fps),
	}
	if proto == "rdp" {
		values["username"] = info.Username
		values["security"] = "nla"
		values["resize-method"] = "display-update"
	}
	argsValues := make([]string, 0, len(names)+1)
	argsValues = append(argsValues, "args")
	for _, n := range names {
		argsValues = append(argsValues, values[n])
	}

	// 4) size / image / audio / video / args
	for _, s := range []string{
		guacEncodeInstruction("size", strconv.Itoa(width), strconv.Itoa(height)),
		guacEncodeInstruction("image", "image/png", "image/jpeg"),
		guacEncodeInstruction("audio", "-"),
		guacEncodeInstruction("video", "-"),
		guacEncodeInstruction(argsValues...),
	} {
		if err := writeInstr(s); err != nil {
			return err
		}
	}
	if err := flush(); err != nil {
		return err
	}

	// 5) 等待 ready（期间遇到 error 直接带出提示，其余指令忽略）
	for {
		instr, err = guacReadInstruction(r)
		if err != nil {
			return err
		}
		parts = strings.Split(instr, ",")
		if len(parts) > 0 {
			switch parts[0] {
			case "ready":
				return nil
			case "error":
				if len(parts) > 1 && parts[1] != "" {
					return errors.New(parts[1])
				}
				return errGuacProto
			}
		}
	}
}

// guacSendError 向浏览器下发 guacamole error 指令后关闭
func guacSendError(conn *websocket.Conn, msg string) {
	conn.WriteMessage(websocket.TextMessage, []byte(guacEncodeInstruction("error", msg, "SERVER_ERROR")))
	conn.Close()
}

// guacFail 下发错误后短暂等待，确保前端在连接关闭前收到 error 指令（连接由 defer 关闭）
func guacFail(conn *websocket.Conn, msg string) {
	conn.WriteMessage(websocket.TextMessage, []byte(guacEncodeInstruction("error", msg, "SERVER_ERROR")))
	time.Sleep(200 * time.Millisecond)
}

// ===== VNC 分辨率调整（远程桌面顶栏下拉，临时生效） =====
//
// VNC 桌面分辨率由目标机 Xvnc 决定，经 SSH 执行 xrandr 切换；
// 画面变化经 rdpd 的 DesktopSize 编码自动同步前端画布，无需重连。

// sshExecOnce 与目标机建立一次性 SSH 连接并执行命令（复用 SFTP 的认证规则）
func sshExecOnce(server *model.Server, command string) (string, error) {
	return sshExecOnceTimeout(server, command, 15*time.Second)
}

// sshExecOnceTimeout 同 sshExecOnce，但可自定义命令执行超时
// （桌面确认弹窗需等待对方点击，最长 60 秒以上）
func sshExecOnceTimeout(server *model.Server, command string, timeout time.Duration) (string, error) {
	if server.AuthType == "key" {
		if plain, ok := utils.DecryptEx(server.PrivateKey); ok {
			server.PrivateKey = plain
		} else {
			return "", errors.New("服务器私钥无法解密（加密密钥已变更），请在服务器列表中重新编辑并保存")
		}
	} else if plain, ok := utils.DecryptEx(server.Password); ok {
		server.Password = plain
	} else {
		return "", errors.New("服务器密码无法解密（加密密钥已变更），请在服务器列表中重新编辑并保存")
	}
	var authMethods []ssh.AuthMethod
	if server.AuthType == "key" {
		signer, err := ssh.ParsePrivateKey([]byte(server.PrivateKey))
		if err != nil {
			return "", fmt.Errorf("解析私钥失败: %v", err)
		}
		authMethods = append(authMethods, ssh.PublicKeys(signer))
	} else {
		authMethods = append(authMethods, ssh.Password(server.Password))
		authMethods = append(authMethods, ssh.KeyboardInteractive(func(name, instruction string, questions []string, echos []bool) ([]string, error) {
			answers := make([]string, len(questions))
			for i := range questions {
				answers[i] = server.Password
			}
			return answers, nil
		}))
	}
	conn, err := ssh.Dial("tcp", fmt.Sprintf("%s:%d", server.IP, server.Port), &ssh.ClientConfig{
		User:            server.Username,
		Auth:            authMethods,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         10 * time.Second,
	})
	if err != nil {
		return "", fmt.Errorf("连接服务器失败: %v", err)
	}
	defer conn.Close()

	sess, err := conn.NewSession()
	if err != nil {
		return "", err
	}
	defer sess.Close()
	var out, errOut bytes.Buffer
	sess.Stdout = &out
	sess.Stderr = &errOut
	done := make(chan error, 1)
	go func() { done <- sess.Run(command) }()
	select {
	case err = <-done:
	case <-time.After(timeout):
		sess.Close()
		return "", errors.New("命令执行超时")
	}
	if err != nil {
		if msg := strings.TrimSpace(errOut.String()); msg != "" {
			return out.String(), fmt.Errorf("%v: %s", err, msg)
		}
	}
	return out.String(), err
}

// vncDisplayFromPort 由 VNC 端口推算 X display（5901 → :1）。
// SSH 非交互执行没有 DISPLAY 环境，必须显式导出后 xrandr 才能连上 Xvnc。
func vncDisplayFromPort(port int) string {
	d := port - 5900
	if d < 0 {
		d = 0
	}
	return ":" + strconv.Itoa(d)
}

// vncXrandrOutput 解析 xrandr --current 输出：返回（输出名, 当前分辨率, 可用分辨率列表）
func vncXrandrOutput(out string) (output, current string, modes []string) {
	seen := map[string]bool{}
	for _, line := range strings.Split(out, "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		if strings.Contains(line, " connected") && output == "" {
			output = fields[0]
			for _, f := range fields[1:] {
				// 当前模式形如 1024x768+0+0
				if i := strings.IndexByte(f, '+'); i > 0 && strings.Contains(f[:i], "x") {
					current = f[:i]
					break
				}
			}
			continue
		}
		// 模式行：缩进开头的 WxH freq（带 * 为当前，+ 为首选）
		if len(fields) >= 2 && strings.HasPrefix(line, " ") && strings.Contains(fields[0], "x") {
			m := fields[0]
			if !seen[m] {
				seen[m] = true
				modes = append(modes, m)
			}
		}
	}
	return
}

// vncCommonModes 常用分辨率档位（16:9 为主，后跟 16:10）：
// 目标机 Xvnc 自带的模式列表多为 4:3/5:4，这里统一补充常用宽屏档并置于列表
// 前部；选中目标机不存在的档位时经 xrandr --newmode/--addmode 动态添加。
var vncCommonModes = []string{
	"1280x720", "1366x768", "1600x900", "1920x1080", "2560x1440", "3840x2160",
	"1280x800", "1440x900", "1680x1050", "1920x1200",
}

// vncModelines 各档位标准时序（CEA/CVT 60Hz）。Xvnc 为虚拟显示，时序数值仅
// 要求语法合法；真实显卡下这些也是标准值，动态添加同样安全。
var vncModelines = map[string]string{
	"1280x720":  "74.25 1280 1390 1430 1650 720 725 730 750 +hsync +vsync",
	"1366x768":  "85.50 1366 1424 1536 1792 768 771 777 795 +hsync +vsync",
	"1600x900":  "118.25 1600 1696 1856 2112 900 903 908 934 -hsync +vsync",
	"1920x1080": "173.00 1920 2048 2248 2576 1080 1083 1088 1120 -hsync +vsync",
	"2560x1440": "312.25 2560 2752 3040 3520 1440 1443 1448 1493 -hsync +vsync",
	"3840x2160": "594.00 3840 4016 4104 4400 2160 2168 2178 2250 +hsync +vsync",
	"1280x800":  "83.50 1280 1352 1480 1680 800 803 809 831 -hsync +vsync",
	"1440x900":  "106.50 1440 1528 1672 1904 900 903 909 934 -hsync +vsync",
	"1680x1050": "146.25 1680 1784 1960 2240 1050 1053 1059 1089 -hsync +vsync",
	"1920x1200": "193.25 1920 2056 2256 2592 1200 1203 1209 1245 -hsync +vsync",
}

// vncEnsureMode 确保目标机 X 显示上存在指定分辨率模式：未收录时以标准时序
// 经 xrandr --newmode/--addmode 动态添加（对虚拟显示无持久痕迹，随会话消失）。
func vncEnsureMode(server *model.Server, display, output, mode string) error {
	out, err := sshExecOnce(server, "export DISPLAY="+display+"; xrandr --current")
	if err != nil {
		return err
	}
	_, _, modes := vncXrandrOutput(out)
	for _, m := range modes {
		if m == mode {
			return nil
		}
	}
	ml, ok := vncModelines[mode]
	if !ok {
		// 非预设档位：生成语法合法的简化时序（Xvnc 不校验数值）
		var w, h int
		if _, err := fmt.Sscanf(mode, "%dx%d", &w, &h); err != nil || w <= 0 || h <= 0 {
			return fmt.Errorf("无效分辨率 %s", mode)
		}
		ml = fmt.Sprintf("%.2f %d %d %d %d %d %d %d %d -hsync +vsync",
			float64(w)*float64(h)*60*1.15/1e6, w, w+40, w+80, w+320, h, h+3, h+6, h+40)
	}
	// 模式可能已存在（未挂载到当前 output），newmode 报错可忽略，以 addmode 为准
	_, _ = sshExecOnce(server, fmt.Sprintf("export DISPLAY=%s; xrandr --newmode %s %s", display, mode, ml))
	if _, err := sshExecOnce(server, fmt.Sprintf("export DISPLAY=%s; xrandr --addmode %s %s", display, output, mode)); err != nil {
		return err
	}
	return nil
}

// VNCResolutions 查询目标机 VNC 桌面可用分辨率列表
func VNCResolutions(c *gin.Context) {
	id64, err := strconv.ParseUint(c.Query("server_id"), 10, 64)
	if err != nil || id64 == 0 {
		utils.Fail(c, "参数错误")
		return
	}
	server, ok := checkServerPermission(c, uint(id64))
	if !ok {
		return
	}
	if server.OsType == "windows" {
		utils.Fail(c, "仅 Linux（VNC）远程桌面支持调整分辨率")
		return
	}
	if server.VncPort <= 0 {
		utils.Fail(c, "该服务器未配置 VNC 端口")
		return
	}
	out, err := sshExecOnce(server, "export DISPLAY="+vncDisplayFromPort(server.VncPort)+"; xrandr --current")
	if err != nil {
		utils.Fail(c, "查询分辨率失败："+err.Error())
		return
	}
	_, current, modes := vncXrandrOutput(out)
	if len(modes) == 0 {
		utils.Fail(c, "未获取到可用分辨率（目标机 xrandr 不可用或 VNC 桌面异常）")
		return
	}
	// 常用宽屏档（16:9 为主）置于前部，目标机自带模式去重后追加在后：
	// 目标机 Xvnc 自带列表多为 4:3/5:4，避免下拉里 16:9 档位被淹没
	merged := make([]string, 0, len(modes)+len(vncCommonModes))
	seen := map[string]bool{}
	for _, m := range append(append([]string{}, vncCommonModes...), modes...) {
		if !seen[m] {
			seen[m] = true
			merged = append(merged, m)
		}
	}
	if current == "" && len(modes) > 0 {
		current = modes[0]
	}
	utils.Success(c, gin.H{"current": current, "modes": merged})
}

// VNCResolution 调整目标机 VNC 桌面分辨率（SSH 执行 xrandr，仅当前会话临时生效）
func VNCResolution(c *gin.Context) {
	var req struct {
		ServerID   uint   `json:"server_id"`
		Resolution string `json:"resolution"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.ServerID == 0 {
		utils.Fail(c, "参数错误")
		return
	}
	// 分辨率仅允许 WxH 数字格式（拼入 SSH 命令，必须严格校验防注入）
	if !regexp.MustCompile(`^\d{2,5}x\d{2,5}$`).MatchString(req.Resolution) {
		utils.Fail(c, "分辨率格式错误")
		return
	}
	server, ok := checkServerPermission(c, req.ServerID)
	if !ok {
		return
	}
	if server.OsType == "windows" {
		utils.Fail(c, "仅 Linux（VNC）远程桌面支持调整分辨率")
		return
	}
	if server.VncPort <= 0 {
		utils.Fail(c, "该服务器未配置 VNC 端口")
		return
	}
	display := vncDisplayFromPort(server.VncPort)
	out, err := sshExecOnce(server, "export DISPLAY="+display+"; xrandr --current")
	if err != nil {
		utils.Fail(c, "查询显示信息失败："+err.Error())
		return
	}
	output, _, _ := vncXrandrOutput(out)
	if output == "" {
		output = "VNC-0"
	}
	// 目标机未收录该档位（Xvnc 自带列表多为 4:3/5:4）时动态添加，宽屏档由此生效
	if err := vncEnsureMode(server, display, output, req.Resolution); err != nil {
		utils.Fail(c, "添加分辨率模式失败："+err.Error())
		return
	}
	// output/mode 均来自白名单校验或 xrandr 自身输出，无注入面
	cmd := fmt.Sprintf("export DISPLAY=%s; xrandr --output %s --mode %s", display, output, req.Resolution)
	if _, err := sshExecOnce(server, cmd); err != nil {
		utils.Fail(c, "切换分辨率失败："+err.Error()+"（请确认所选分辨率在 xrandr 支持列表中）")
		return
	}
	utils.Success(c, gin.H{"resolution": req.Resolution})
}
