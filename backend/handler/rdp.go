package handler

import (
	"bufio"
	"errors"
	"io"
	"log"
	"net"
	"net/url"
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

	// RDP 需密码认证（NLA）
	plain, ok := utils.DecryptEx(server.Password)
	if !ok || plain == "" {
		utils.Fail(c, "该服务器未配置密码，无法使用远程桌面（RDP 需密码认证）")
		return
	}

	token := uuid.New().String()
	rdpTokens.Store(token, rdpTokenInfo{
		ServerID: server.ID,
		UserID:   userID.(uint),
		IP:       server.IP,
		Port:     server.Port,
		Username: server.Username,
		Password: plain,
		ExpireAt: time.Now().Add(60 * time.Second),
	})
	utils.Success(c, gin.H{"token": token})
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
		log.Printf("RDP 内置引擎：server=%d 前端请求 %dx%d", info.ServerID, w, h)
		builtinRDPSession(conn, info, w, h)
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
	if err := guacHandshake(guacReader, guacWriter, guacWriter, info, w, h); err != nil {
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
func guacHandshake(r *bufio.Reader, w io.Writer, flusher interface{ Flush() error }, info rdpTokenInfo, width, height int) error {
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
	// 1) select rdp
	if err := writeInstr(guacEncodeInstruction("select", "rdp")); err != nil {
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

	// 3) 按参数名回填值（未知参数留空，guacd 忽略空值）
	values := map[string]string{
		"hostname":      info.IP,
		"port":          strconv.Itoa(info.Port),
		"username":      info.Username,
		"password":      info.Password,
		"security":      "nla",
		"ignore-cert":   "true",
		"resize-method": "display-update",
		"client-name":   "QinJiaGateway",
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
