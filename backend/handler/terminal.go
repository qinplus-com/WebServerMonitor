package handler

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"qinjiagateway/dao"
	"qinjiagateway/model"
	sshmgr "qinjiagateway/ssh"
	"qinjiagateway/utils"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024 * 4,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

// TerminalConnect 发起SSH连接，返回session_id
func TerminalConnect(c *gin.Context) {
	var req struct {
		ServerID uint `json:"server_id" binding:"required"`
		Cols     int  `json:"cols"`
		Rows     int  `json:"rows"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Fail(c, "参数错误")
		return
	}
	if req.Cols == 0 {
		req.Cols = 120
	}
	if req.Rows == 0 {
		req.Rows = 40
	}
	userID, _ := c.Get("user_id")
	role, _ := c.Get("role")

	// 校验服务器权限
	var server model.Server
	if err := dao.DB.First(&server, req.ServerID).Error; err != nil {
		utils.Fail(c, "服务器不存在")
		return
	}
	if role != "admin" {
		var cnt int64
		dao.DB.Model(&model.UserServer{}).Where("user_id = ? AND server_id = ?", userID, req.ServerID).Count(&cnt)
		if cnt == 0 {
			utils.Fail(c, "无该服务器访问权限")
			return
		}
	}

	sessionID := uuid.New().String()
	sess, err := sshmgr.Connect(sessionID, userID.(uint), &server, req.Cols, req.Rows)
	if err != nil {
		utils.Fail(c, err.Error())
		return
	}

	// 启动读取goroutine：将SSH输出推送到WebSocket由连接后处理
	go func() {
		buf := make([]byte, 4096)
		for {
			n, err := sess.Read(buf)
			if err != nil {
				break
			}
			// 必须拷贝：buf 会被下一次 Read 覆写，而消费方（WS 写入 goroutine）是异步的
			data := make([]byte, n)
			copy(data, buf[:n])
			// 缓存输出，等待WebSocket连接后读取
			pushStdout(sessionID, data)
		}
		sshmgr.RemoveSession(sessionID)
	}()

	utils.Success(c, gin.H{"session_id": sessionID})
}

// stdoutBuffer 临时缓存SSH输出，直到WebSocket连接建立
var stdoutBuffers = make(map[string][]byte)
var bufMu = make(map[string]chan []byte)

func pushStdout(sessionID string, data []byte) {
	ch, ok := bufMu[sessionID]
	if ok {
		select {
		case ch <- data:
		default:
		}
		return
	}
	stdoutBuffers[sessionID] = append(stdoutBuffers[sessionID], data...)
}

// TerminalWebSocket WebSocket终端长连接
func TerminalWebSocket(c *gin.Context) {
	sessionID := c.Query("session_id")
	if sessionID == "" {
		c.JSON(400, gin.H{"msg": "缺少session_id"})
		return
	}
	sess := sshmgr.GetSession(sessionID)
	if sess == nil {
		c.JSON(400, gin.H{"msg": "会话不存在或已关闭"})
		return
	}

	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Println("WebSocket升级失败:", err)
		return
	}
	defer conn.Close()

	// 创建输出channel
	outCh := make(chan []byte, 256)
	bufMu[sessionID] = outCh
	defer func() {
		delete(bufMu, sessionID)
	}()

	// 发送缓存的数据
	if buf := stdoutBuffers[sessionID]; len(buf) > 0 {
		conn.WriteMessage(websocket.TextMessage, []byte(`{"type":"stdout","data":`+jsonEscape(string(buf))+`}`))
		delete(stdoutBuffers, sessionID)
	}

	// 读取SSH输出推送到WebSocket
	go func() {
		for data := range outCh {
			msg, _ := json.Marshal(gin.H{"type": "stdout", "data": string(data)})
			conn.WriteMessage(websocket.TextMessage, msg)
		}
	}()

	// 心跳超时检测
	lastPing := time.Now()
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			if time.Since(lastPing) > 90*time.Second {
				conn.Close()
				return
			}
		}
	}()

	// 读取前端消息
	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			break
		}
		var data struct {
			Type string `json:"type"`
			Data string `json:"data"`
			Cols int    `json:"cols"`
			Rows int    `json:"rows"`
		}
		if err := json.Unmarshal(msg, &data); err != nil {
			continue
		}
		switch data.Type {
		case "command":
			if sess.IsClosed() {
				break
			}
			sess.Write([]byte(data.Data))
			// 记录命令日志（简单记录输入的命令）
			if len(data.Data) > 0 && data.Data != "\r" && data.Data != "\n" {
				go recordCommand(sess, data.Data)
			}
		case "resize":
			sess.Resize(data.Cols, data.Rows)
		case "ping":
			lastPing = time.Now()
			resp, _ := json.Marshal(gin.H{"type": "pong"})
			conn.WriteMessage(websocket.TextMessage, resp)
		}
	}

	sshmgr.RemoveSession(sessionID)
}

func recordCommand(sess *sshmgr.Session, cmd string) {
	// 过滤控制字符，只记录可打印命令
	printable := ""
	for _, r := range cmd {
		if r >= 32 || r == '\r' || r == '\n' {
			printable += string(r)
		}
	}
	if len(printable) > 0 && printable != "\r" && printable != "\n" {
		dao.DB.Create(&model.CommandLog{
			UserID:     sess.UserID,
			Username:   getUsername(sess.UserID),
			ServerID:   sess.ServerID,
			ServerName: sess.ServerName,
			SessionID:  sess.ID,
			Command:    printable,
		})
	}
}

func getUsername(userID uint) string {
	var u model.User
	dao.DB.First(&u, userID)
	return u.Username
}

// TerminalClose 关闭终端会话
func TerminalClose(c *gin.Context) {
	var req struct {
		SessionID string `json:"session_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Fail(c, "参数错误")
		return
	}
	sshmgr.RemoveSession(req.SessionID)
	utils.Success(c, nil)
}

// TerminalHistory 获取终端历史命令
func TerminalHistory(c *gin.Context) {
	sessionID := c.Query("session_id")
	var logs []model.CommandLog
	query := dao.DB.Model(&model.CommandLog{})
	if sessionID != "" {
		query = query.Where("session_id = ?", sessionID)
	}
	query.Order("id DESC").Limit(100).Find(&logs)
	utils.Success(c, logs)
}

// SessionOnline 在线会话列表
func SessionOnline(c *gin.Context) {
	utils.Success(c, sshmgr.GetAllSessions())
}

// SessionKick 强制关闭会话
func SessionKick(c *gin.Context) {
	var req struct {
		SessionID string `json:"session_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Fail(c, "参数错误")
		return
	}
	sshmgr.RemoveSession(req.SessionID)
	utils.Success(c, nil)
}

func jsonEscape(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}
