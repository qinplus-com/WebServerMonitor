package main

// VNC（RFB）会话：纯标准库实现 RFB 3.3/3.7/3.8 客户端，
// 用于连接已装桌面环境的 Linux（tigervnc 等）的远程桌面：
//
//	画面：32bpp + Hextile/CopyRect/Raw 编码 → 帧缓冲合成 → PNG tile 增量推送
//	输入：guacamole key/mouse 指令 → RFB KeyEvent/PointerEvent（keysym/掩码直传）
//
// 与 rdp.go 共用 guacWriter 与 tile 渲染管线（脏块 + 定帧批量推送）。
// 仅依赖标准库（VNC 认证用 crypto/des），不引入第三方依赖。

import (
	"bufio"
	"bytes"
	"crypto/des"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"io"
	"log"
	"math/bits"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"sync/atomic"
	"time"
	"unicode/utf8"
)

const (
	vncDialTimeout      = 10 * time.Second
	vncHandshakeTimeout = 15 * time.Second
)

// 客户端 -> 服务器 消息类型
const (
	vncMsgSetPixelFormat           = 0
	vncMsgSetEncodings             = 2
	vncMsgFramebufferUpdateRequest = 3
	vncMsgKeyEvent                 = 4
	vncMsgPointerEvent             = 5
	vncMsgClientCutText            = 6
)

// 服务器 -> 客户端 消息类型
const (
	vncMsgFramebufferUpdate   = 0
	vncMsgSetColourMapEntries = 1
	vncMsgBell                = 2
	vncMsgServerCutText       = 3
)

// RFB 编码类型
const (
	vncEncRaw         = 0
	vncEncCopyRect    = 1
	vncEncHextile     = 5
	vncEncDesktopSize = -223
)

// Hextile 子编码位
const (
	hextileRaw              = 1
	hextileBgSpecified      = 2
	hextileFgSpecified      = 4
	hextileAnySubrects      = 8
	hextileSubrectsColoured = 16
)

type vncSession struct {
	params   connParams
	w        *guacWriter
	supports map[string]bool

	conn    net.Conn
	writeMu sync.Mutex // 保护对 conn 的写（读循环与输入指令并发）
	live    bool
	closed  chan struct{}
	closeOnce sync.Once

	// 帧缓冲与脏块（与 rdp.session 相同的推送管线）
	fb      *image.RGBA
	fbMu    sync.Mutex
	fbW     int
	fbH     int
	tilesX  int
	tilesY  int
	tiles   []bool
	// sentHash 各 tile 最近一次成功推送内容的 CRC32C（0=从未发送），用于内容去重
	sentHash []uint64
	nFrames  atomic.Int64

	// 渲染唤醒：帧更新到达时即时触发 renderLoop（限流见 minFrameGap）
	kick       chan struct{}
	lastRender time.Time

	// Hextile 背景/前景色跨 tile 保持
	bg [4]byte
	fg [4]byte

	// colored subrect 字段顺序（首个 colored subrect 自动判别）：
	// RFC 6143 为 [坐标2B][颜色4B]，部分定制服务器（如 waf 系列）为 [颜色4B][坐标2B]
	subrectFmtKnown   bool
	subrectColorFirst bool

	// 剪贴板：浏览器 clipboard 流缓冲（clipboard → blob* → end → ClientCutText）
	clipBufs map[int][]byte

	// H.264 视频通道（h264.go）：codec=h264 且 ffmpeg 可用时启用，renderLoop
	// 整帧交 ffmpeg 帧间压缩；失效自动回退 tile 管线。vidRestart 置位（DesktopSize
	// 分辨率变更/前端 keyframe 请求）时下一渲染帧重建编码器以强制 IDR。
	vid        videoPipe
	vidWanted  bool
	vidDirty   atomic.Bool
	vidRestart atomic.Bool
	// vidHash 各 tile 最近一次"已实际编码"内容的 CRC32C 指纹（内容缓存），
	// 语义与 rdp.session.vidHash 相同：脏块命中即清脏跳过整帧编码
	vidHash []uint64

	// 智能无损状态（仅 renderLoop 协程读写）：lastVideoChange 最近一次
	// 实际编码视频帧的时刻；losslessDone 当前静止期是否已补发无损帧
	lastVideoChange time.Time
	losslessDone    bool

	// 调试转储挂起标记（仅 readLoop 协程读写）：DesktopSize 变更后转储一次 fb
	dumpAfterUpdate bool
}

func newVNCSession(p connParams, w *guacWriter, mimes []string) *vncSession {
	s := &vncSession{
		params:   p,
		w:        w,
		supports: map[string]bool{},
		clipBufs: map[int][]byte{},
		closed:   make(chan struct{}),
		kick:     make(chan struct{}, 1),
	}
	for _, m := range mimes {
		s.supports[m] = true
	}
	return s
}

func (s *vncSession) stop() {
	s.closeOnce.Do(func() {
		close(s.closed)
		s.vid.stop() // 停止 H.264 编码管线（幂等）
		if s.conn != nil {
			_ = s.conn.Close() // 幂等：连接失败时 Dial 已自行清理
		}
	})
}

func (s *vncSession) fail(msg string) {
	if msg != "" {
		_ = s.w.write("error", msg, "SERVER_ERROR")
	}
	s.stop()
}

// run 建立 VNC 连接并阻塞至会话结束。
// 返回错误 = 连接阶段失败（已通过 fail 下发 error 指令）。
func (s *vncSession) run() error {
	defer s.stop()

	if err := s.connect(); err != nil {
		s.fail(err.Error())
		return err
	}
	s.live = true

	s.fbMu.Lock()
	s.initFbLocked()
	s.fbMu.Unlock()
	log.Printf("rdpd VNC 会话就绪 %s:%d 桌面 %dx%d", s.params.host, s.params.port, s.fbW, s.fbH)
	_ = s.w.write("size", "0", strconv.Itoa(s.fbW), strconv.Itoa(s.fbH))
	// 视频模式初始化：前端请求 codec=h264 且 ffmpeg 可用时启用；失败自动回退 tile。
	// 首帧画面随首个帧更新响应到达后即推送
	if s.params.codec == "h264" {
		if err := s.vid.start(s.fbW, s.fbH, s.params.fps, crfForQuality(s.params.quality), s.onVideoAU); err == nil {
			s.vidWanted = true
			s.vidDirty.Store(true)
			s.lastVideoChange = time.Now()
			log.Printf("[vnc video] 视频通道已启用（libx264 %dx%d %dfps crf%d %s）",
				s.fbW, s.fbH, s.params.fps, crfForQuality(s.params.quality), s.params.quality)
		} else {
			log.Printf("[vnc video] 视频通道不可用，使用 tile 管线: %v", err)
		}
	}
	go s.renderLoop()
	go s.syncLoop()

	// 首次全量帧更新请求，之后每处理完一帧再请求增量（乒乓驱动）
	s.sendFBURequest(false)
	return s.readLoop()
}

// connect 拨号并完成 RFB 握手（版本 / 安全类型 / 认证 / 初始化 / 像素格式与编码）
func (s *vncSession) connect() error {
	addr := net.JoinHostPort(s.params.host, strconv.Itoa(s.params.port))
	conn, err := net.DialTimeout("tcp", addr, vncDialTimeout)
	if err != nil {
		return errors.New("无法连接 VNC 服务 " + addr + "：" + err.Error() + "（请确认目标机已启动 VNC 服务且防火墙已放行该端口）")
	}
	s.conn = conn
	_ = conn.SetDeadline(time.Now().Add(vncHandshakeTimeout))

	// 1) 版本握手：取服务器版本与 3.8 的较小者
	var ver [12]byte
	if _, err := io.ReadFull(conn, ver[:]); err != nil {
		return errors.New("VNC 版本握手失败：" + err.Error())
	}
	if string(ver[:4]) != "RFB " {
		return errors.New("目标端口不是 VNC/RFB 服务（" + string(ver[:]) + "...），请检查 VNC 端口配置")
	}
	// RFB 版本串形如 "RFB 003.008\n"：主版本第 7 字节、次版本第 11 字节
	// （此前按三位数字解析，'.'-'0' 产生负偏移导致 smaj 恒为 28、误判走 3.8 分支，
	//  对仅支持 3.3 的服务器会造成安全类型解析错乱直至握手超时）
	if ver[6] < '0' || ver[6] > '9' || ver[10] < '0' || ver[10] > '9' {
		return errors.New("VNC 版本串无效：" + string(ver[:]))
	}
	smaj := int(ver[6] - '0')
	smin := int(ver[10] - '0')
	rmin := 3
	switch {
	case smaj > 3 || (smaj == 3 && smin >= 8):
		rmin = 8
	case smaj == 3 && smin == 7:
		rmin = 7
	}
	if _, err := conn.Write([]byte(fmt.Sprintf("RFB 003.%03d\n", rmin))); err != nil {
		return err
	}

	// 2) 安全类型协商
	var chosen uint8
	if rmin == 3 {
		// 3.3：服务器直接下发安全类型（u32）
		var t [4]byte
		if _, err := io.ReadFull(conn, t[:]); err != nil {
			return err
		}
		chosen = uint8(binary.BigEndian.Uint32(t[:]))
		if chosen == 0 {
			reason, rerr := vncReadReason(conn)
			if rerr != nil {
				return rerr
			}
			return errors.New("VNC 连接被拒绝：" + reason)
		}
		if chosen != 1 && chosen != 2 {
			return errors.New("VNC 服务器要求不支持的安全类型 " + strconv.Itoa(int(chosen)))
		}
	} else {
		var n [1]byte
		if _, err := io.ReadFull(conn, n[:]); err != nil {
			return err
		}
		if n[0] == 0 {
			reason, rerr := vncReadReason(conn)
			if rerr != nil {
				return rerr
			}
			return errors.New("VNC 连接被拒绝：" + reason)
		}
		types := make([]byte, int(n[0]))
		if _, err := io.ReadFull(conn, types); err != nil {
			return err
		}
		hasNone, hasAuth := false, false
		for _, t := range types {
			if t == 1 {
				hasNone = true
			}
			if t == 2 {
				hasAuth = true
			}
		}
		// 有密码优先走 VNC 认证，否则尝试 None
		if hasAuth && s.params.password != "" {
			chosen = 2
		} else if hasNone {
			chosen = 1
		} else {
			return errors.New("该 VNC 服务要求密码认证：请在目标机执行 vncpasswd 设置密码（与服务器编辑中填写的登录密码一致即可）")
		}
		if _, err := conn.Write([]byte{chosen}); err != nil {
			return err
		}
	}

	// 3) VNC 认证（challenge/response，DES-ECB）
	if chosen == 2 {
		if s.params.password == "" {
			return errors.New("该 VNC 服务需要密码认证：请在服务器编辑中填写密码（与目标机 vncpasswd 一致）")
		}
		var challenge [16]byte
		if _, err := io.ReadFull(conn, challenge[:]); err != nil {
			return errors.New("VNC 认证握手失败：" + err.Error())
		}
		resp := vncChallengeResponse(challenge[:], []byte(s.params.password))
		if _, err := conn.Write(resp[:]); err != nil {
			return err
		}
	}

	// 4) 3.8 认证结果（0=成功；失败时跟原因串）
	if rmin == 8 {
		var res [4]byte
		if _, err := io.ReadFull(conn, res[:]); err != nil {
			return err
		}
		if binary.BigEndian.Uint32(res[:]) != 0 {
			if reason, rerr := vncReadReason(conn); rerr == nil && reason != "" {
				return errors.New("VNC 认证失败：" + reason)
			}
			return errors.New("VNC 认证失败：请核对服务器密码与目标机 vncpasswd 是否一致")
		}
	}

	// 5) ClientInit：共享桌面
	if _, err := conn.Write([]byte{1}); err != nil {
		return err
	}

	// 6) ServerInit：桌面尺寸 + 像素格式 + 名称
	var si [24]byte
	if _, err := io.ReadFull(conn, si[:]); err != nil {
		return errors.New("VNC 会话初始化失败：" + err.Error())
	}
	s.fbW = int(binary.BigEndian.Uint16(si[0:2]))
	s.fbH = int(binary.BigEndian.Uint16(si[2:4]))
	nameLen := binary.BigEndian.Uint32(si[20:24])
	if nameLen > 4*1024 {
		nameLen = 4 * 1024
	}
	name := make([]byte, nameLen)
	if _, err := io.ReadFull(conn, name); err != nil {
		return err
	}
	// 帧缓冲宽高向上取偶：H.264 视频通道要求 yuv420p 偶数尺寸（至多 1px 黑边不可见）
	s.fbW, s.fbH = evenAlign(s.fbW), evenAlign(s.fbH)
	if s.fbW <= 0 || s.fbH <= 0 {
		return errors.New("VNC 服务器回报了无效的桌面尺寸")
	}

	// 7) SetPixelFormat：32bpp/depth24/小端/真彩，max 255、shift R=0 G=8 B=16，
	//    每像素内存字节序恰为 R,G,B,0，可直接写入 image.RGBA（alpha 补 255）
	pf := make([]byte, 20)
	pf[0] = vncMsgSetPixelFormat
	pf[4] = 32  // bits-per-pixel
	pf[5] = 24  // depth
	pf[6] = 0   // big-endian flag：小端
	pf[7] = 1   // true colour
	binary.BigEndian.PutUint16(pf[8:], 255)   // red-max
	binary.BigEndian.PutUint16(pf[10:], 255)  // green-max
	binary.BigEndian.PutUint16(pf[12:], 255)  // blue-max
	pf[14] = 0  // red-shift
	pf[15] = 8  // green-shift
	pf[16] = 16 // blue-shift
	if _, err := conn.Write(pf); err != nil {
		return err
	}

	// 8) SetEncodings：DesktopSize(-223) + Hextile(5) + CopyRect(1) + Raw(0)
	encs := make([]byte, 4+4*4)
	encs[0] = vncMsgSetEncodings
	binary.BigEndian.PutUint16(encs[2:], 4)
	for i, e := range []int32{vncEncDesktopSize, vncEncHextile, vncEncCopyRect, vncEncRaw} {
		binary.BigEndian.PutUint32(encs[4+i*4:], uint32(e))
	}
	if _, err := conn.Write(encs); err != nil {
		return err
	}

	// 握手完成，清除超时（长会话）
	_ = conn.SetDeadline(time.Time{})
	return nil
}

// vncChallengeResponse 计算 VNC 认证响应：以密码为 DES 密钥（截断/补零到 8 字节、
// 逐位反转，RFB 规范），对 16 字节 challenge 做 DES-ECB 加密。
func vncChallengeResponse(challenge, password []byte) [16]byte {
	var key [8]byte
	for i := 0; i < 8 && i < len(password); i++ {
		key[i] = bits.Reverse8(password[i])
	}
	var out [16]byte
	block, _ := des.NewCipher(key[:])
	block.Encrypt(out[0:8], challenge[0:8])
	block.Encrypt(out[8:16], challenge[8:16])
	return out
}

// vncReadReason 读取 u32 长度 + 文本的原因串（3.8 认证失败 / 3.7 类型表为空）
func vncReadReason(conn net.Conn) (string, error) {
	var l [4]byte
	if _, err := io.ReadFull(conn, l[:]); err != nil {
		return "", err
	}
	n := binary.BigEndian.Uint32(l[:])
	if n > 4*1024 {
		n = 4 * 1024
	}
	b := make([]byte, n)
	if _, err := io.ReadFull(conn, b); err != nil {
		return "", err
	}
	return string(b), nil
}

// ===== 服务器消息读循环 =====

func (s *vncSession) readLoop() error {
	r := bufio.NewReaderSize(s.conn, 64*1024)
	for {
		select {
		case <-s.closed:
			return nil
		default:
		}
		var t [1]byte
		if _, err := io.ReadFull(r, t[:]); err != nil {
			return err
		}
		switch t[0] {
		case vncMsgFramebufferUpdate:
			if err := s.handleFramebufferUpdate(r); err != nil {
				return err
			}
			if s.dumpAfterUpdate {
				s.dumpAfterUpdate = false
				s.vncFbDump("resize")
			}
			// 乒乓驱动：处理完一帧立即请求增量更新
			s.sendFBURequest(true)
			// 新画面已入帧缓冲：唤醒渲染循环立即推送（渲染侧按 minFrameGap 限流）
			select {
			case s.kick <- struct{}{}:
			default:
			}
		case vncMsgSetColourMapEntries:
			// 读掉忽略（1 pad + 2 first + 2 count + 6n）
			var hdr [5]byte
			if _, err := io.ReadFull(r, hdr[:]); err != nil {
				return err
			}
			n := int(binary.BigEndian.Uint16(hdr[3:5]))
			if _, err := io.CopyN(io.Discard, r, int64(n)*6); err != nil {
				return err
			}
		case vncMsgBell:
			// 忽略
		case vncMsgServerCutText:
			var hdr [7]byte // 3 pad + 4 len
			if _, err := io.ReadFull(r, hdr[:]); err != nil {
				return err
			}
			n := int64(binary.BigEndian.Uint32(hdr[3:7]))
			if n > 8*1024*1024 { // 异常大文本丢弃（防内存炸弹）
				if _, err := io.CopyN(io.Discard, r, n); err != nil {
					return err
				}
				continue
			}
			buf := make([]byte, n)
			if _, err := io.ReadFull(r, buf); err != nil {
				return err
			}
			// RFB 规范文本为 Latin-1；主流实现（TigerVNC 等）实际发 UTF-8，
			// 先按 UTF-8 校验，非法则按 Latin-1 逐字节转换
			text := string(buf)
			if !utf8.Valid(buf) {
				rs := make([]rune, len(buf))
				for i, c := range buf {
					rs[i] = rune(c)
				}
				text = string(rs)
			}
			// 远端剪贴板 → 浏览器（guac clipboard 流）
			emitGuacClipboard(s.w, text)
		default:
			return errors.New("VNC 服务器发送了未知消息类型 " + strconv.Itoa(int(t[0])))
		}
	}
}

func (s *vncSession) handleFramebufferUpdate(r *bufio.Reader) error {
	var hdr [3]byte // 1 pad + 2 nRects
	if _, err := io.ReadFull(r, hdr[:]); err != nil {
		return err
	}
	n := int(binary.BigEndian.Uint16(hdr[1:3]))
	if n == 0 {
		return nil
	}
	// 读循环是唯一的帧缓冲写方，renderLoop 只读；持锁完成整个 update
	s.fbMu.Lock()
	defer s.fbMu.Unlock()
	if s.fb == nil {
		return errors.New("VNC 帧缓冲未初始化")
	}
	for i := 0; i < n; i++ {
		var rh [12]byte
		if _, err := io.ReadFull(r, rh[:]); err != nil {
			return err
		}
		x := int(binary.BigEndian.Uint16(rh[0:2]))
		y := int(binary.BigEndian.Uint16(rh[2:4]))
		w := int(binary.BigEndian.Uint16(rh[4:6]))
		h := int(binary.BigEndian.Uint16(rh[6:8]))
		enc := int32(binary.BigEndian.Uint32(rh[8:12]))
		switch enc {
		case vncEncDesktopSize:
			// 分辨率变更：w/h 位于 rect 头，无载荷
			if err := s.resizeLocked(w, h); err != nil {
				return err
			}
			continue
		case vncEncRaw:
			if err := s.rawRect(r, x, y, w, h); err != nil {
				return err
			}
		case vncEncCopyRect:
			var src [4]byte // srcX s16 + srcY s16
			if _, err := io.ReadFull(r, src[:]); err != nil {
				return err
			}
			s.copyRect(x, y, w, h,
				int(int16(binary.BigEndian.Uint16(src[0:2]))),
				int(int16(binary.BigEndian.Uint16(src[2:4]))))
		case vncEncHextile:
			if err := s.hextileRect(r, x, y, w, h); err != nil {
				return err
			}
		default:
			return errors.New("VNC 服务器使用了未请求的编码 " + strconv.Itoa(int(enc)))
		}
		s.markDirty(x, y, w, h)
	}
	return nil
}

// sendFBURequest 发送帧更新请求
func (s *vncSession) sendFBURequest(incremental bool) {
	s.fbMu.Lock()
	w, h := s.fbW, s.fbH
	s.fbMu.Unlock()
	var msg [10]byte
	msg[0] = vncMsgFramebufferUpdateRequest
	if incremental {
		msg[1] = 1
	}
	binary.BigEndian.PutUint16(msg[6:], uint16(w))
	binary.BigEndian.PutUint16(msg[8:], uint16(h))
	s.writeClient(msg[:])
}

// ===== 输入 =====

// handleInstruction 处理浏览器方向的一条 Guacamole 指令。
// guac 的 keysym 即 X keysym、鼠标掩码与 RFB button mask 完全一致，均直传。
func (s *vncSession) handleInstruction(op string, args []string) {
	atoi := func(i int) int {
		if i < len(args) {
			v, _ := strconv.Atoi(args[i])
			return v
		}
		return 0
	}
	switch op {
	case "key":
		if !s.live {
			return
		}
		var msg [8]byte
		msg[0] = vncMsgKeyEvent
		if atoi(1) != 0 {
			msg[1] = 1
		}
		binary.BigEndian.PutUint32(msg[4:], uint32(atoi(0)))
		s.writeClient(msg[:])
	case "mouse":
		if !s.live {
			return
		}
		var msg [6]byte
		msg[0] = vncMsgPointerEvent
		msg[1] = byte(atoi(2) & 0xFF)
		binary.BigEndian.PutUint16(msg[2:], uint16(atoi(0)))
		binary.BigEndian.PutUint16(msg[4:], uint16(atoi(1)))
		s.writeClient(msg[:])
	case "disconnect":
		s.stop()
	case "clipboard":
		// 浏览器剪贴板流开始（后续 blob*/end 携带内容）
		s.clipBufs[atoi(0)] = []byte{}
	case "blob":
		if b, ok := s.clipBufs[atoi(0)]; ok && len(args) > 1 && len(b) < 8*1024*1024 {
			if raw, err := base64.StdEncoding.DecodeString(args[1]); err == nil {
				s.clipBufs[atoi(0)] = append(b, raw...)
			}
		}
	case "end":
		if b, ok := s.clipBufs[atoi(0)]; ok {
			delete(s.clipBufs, atoi(0))
			if s.live {
				s.sendCutText(string(b))
			}
		}
	case "keyframe":
		// 前端解码异常/标签页重聚焦：重建编码器强制 IDR，重建解码参考链
		if s.vidWanted {
			s.vidRestart.Store(true)
			s.vidDirty.Store(true)
		}
	default:
		// nop / ack / sync 等，忽略
	}
}

// sendCutText 发送 ClientCutText（本地→远端剪贴板）。
// RFB 3.8 规范文本为 Latin-1，主流实现（TigerVNC 等）按 UTF-8 处理，直发 UTF-8。
func (s *vncSession) sendCutText(text string) {
	msg := make([]byte, 8+len(text))
	msg[0] = vncMsgClientCutText
	binary.BigEndian.PutUint32(msg[4:], uint32(len(text)))
	copy(msg[8:], text)
	s.writeClient(msg)
}

func (s *vncSession) writeClient(b []byte) {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	_, _ = s.conn.Write(b)
}

// ===== 帧缓冲 =====

// initFbLocked 按服务器回报的桌面尺寸初始化帧缓冲（须持有 fbMu）
func (s *vncSession) initFbLocked() {
	s.fb = image.NewRGBA(image.Rect(0, 0, s.fbW, s.fbH))
	s.tilesX = (s.fbW + tileW - 1) / tileW
	s.tilesY = (s.fbH + tileH - 1) / tileH
	s.tiles = make([]bool, s.tilesX*s.tilesY)
	s.sentHash = make([]uint64, s.tilesX*s.tilesY)
	s.vidHash = make([]uint64, s.tilesX*s.tilesY)
	for i := range s.tiles {
		s.tiles[i] = true
	}
}

// resizeLocked DesktopSize：重建帧缓冲并全量标脏（须持有 fbMu）
func (s *vncSession) resizeLocked(w, h int) error {
	// 宽高向上取偶：H.264 视频通道要求 yuv420p 偶数尺寸（对齐后再比较，避免
	// 服务器反复下发奇数尺寸导致每次都触发重建）
	w, h = evenAlign(w), evenAlign(h)
	if w <= 0 || h <= 0 || (w == s.fbW && h == s.fbH) {
		return nil
	}
	newFB := image.NewRGBA(image.Rect(0, 0, w, h))
	draw.Draw(newFB, newFB.Bounds(), s.fb, image.Point{}, draw.Src)
	s.fb = newFB
	s.fbW, s.fbH = w, h
	s.tilesX = (s.fbW + tileW - 1) / tileW
	s.tilesY = (s.fbH + tileH - 1) / tileH
	s.tiles = make([]bool, s.tilesX*s.tilesY)
	s.sentHash = make([]uint64, s.tilesX*s.tilesY)
	s.vidHash = make([]uint64, s.tilesX*s.tilesY)
	for i := range s.tiles {
		s.tiles[i] = true
	}
	log.Printf("rdpd VNC 桌面分辨率变更为 %dx%d", w, h)
	s.dumpAfterUpdate = true
	// 视频模式：分辨率变化须重建编码器（新尺寸 + 强制 IDR）；补一次 sync
	// 让前端立即 flush 画布 resize（否则最大延迟一个 sync 周期才重绘）
	if s.vidWanted {
		s.vidRestart.Store(true)
	}
	s.vidDirty.Store(true)
	if err := s.w.write("size", "0", strconv.Itoa(w), strconv.Itoa(h)); err != nil {
		return err
	}
	_ = s.w.write("sync", strconv.FormatInt(time.Now().UnixMilli(), 10))
	return nil
}

// renderGap 渲染帧间隔（1/fps，帧率上限由握手参数 fps 决定，30/60）
func (s *vncSession) renderGap() time.Duration {
	return time.Second / time.Duration(fpsForSession(s.params.fps))
}

// videoPush 视频模式渲染步：必要时重建编码器（DesktopSize/关键帧请求），
// 画面有变化时把整帧 RGBA 写入 ffmpeg。返回 (ok, changed)：ok=false 表示
// 编码管线失效（回退 tile）；changed=true 表示本帧实际写入了编码器。
// 写入前先做脏块内容去重（vidHash 缓存）：与上次已编码帧一致的块直接清脏，
// 无变化重绘不再触发整帧拷贝与编码。与 rdp.session.videoPush 同构。
func (s *vncSession) videoPush() (bool, bool) {
	s.fbMu.Lock()
	restart := s.vidRestart.Swap(false)
	dirty := s.vidDirty.Swap(false)
	w, h := s.fbW, s.fbH
	var pix []byte
	var idxs []int
	var crcs []uint64
	if dirty && !restart && s.vid.active() && s.fb != nil {
		any := false
		for i := range s.tiles {
			if !s.tiles[i] {
				continue
			}
			c := tileCRC(s.fb, s.tilesX, i)
			if s.vidHash[i] == c {
				s.tiles[i] = false // 内容未变：清脏跳过（缓存命中）
				continue
			}
			any = true
			s.tiles[i] = false
			idxs = append(idxs, i)
			crcs = append(crcs, c)
		}
		if !any {
			dirty = false
		} else {
			pix = make([]byte, len(s.fb.Pix))
			copy(pix, s.fb.Pix) // 锁内拷贝快照，锁外写入
		}
	}
	// 重建编码器时同步抓整帧快照与全帧指纹：keyframe 请求常发生在静止画面
	// （tiles 已全部被 vidHash 清脏），若仅置 vidDirty，下一帧会因无脏块而
	// 吞掉 IDR，前端将永久等不到关键帧直至画面再次变化。锁内备好快照，锁外强推。
	var allCRCs []uint64
	if restart && s.fb != nil {
		pix = make([]byte, len(s.fb.Pix))
		copy(pix, s.fb.Pix)
		allCRCs = make([]uint64, s.tilesX*s.tilesY)
		for i := range allCRCs {
			allCRCs[i] = tileCRC(s.fb, s.tilesX, i)
		}
	}
	s.fbMu.Unlock()

	if restart {
		// 编码器换代：内容缓存整体作废（置零防误命中；vidHash 仅 renderLoop
		// 协程读写，无需加锁）
		for i := range s.vidHash {
			s.vidHash[i] = 0
		}
		if err := s.vid.start(w, h, s.params.fps, crfForQuality(s.params.quality), s.onVideoAU); err != nil {
			log.Printf("[vnc video] 编码器重建失败: %v", err)
			return false, false
		}
		if pix != nil {
			// 强制 IDR：无论画面是否静止，重建后立即编码当前整帧推送
			if err := s.vid.writeFrame(pix); err == nil {
				copy(s.vidHash, allCRCs) // 全帧已实际编码：登记指纹作为后续去重基准
				return true, true
			}
			s.vidDirty.Store(true) // 写入失败（罕见）：下一渲染帧重试
		} else {
			s.vidDirty.Store(true) // 会话极早期（fb 未就绪）：等首个帧更新驱动
		}
		return true, false
	}
	if dirty && !s.vid.active() {
		return false, false // 编码器已退出（读出侧 EOF 清空句柄）：立即回退 tile 管线
	}
	if dirty {
		if err := s.vid.writeFrame(pix); err != nil {
			log.Printf("[vnc video] 帧写入失败: %v", err)
			return false, false
		}
		// 该帧已实际编码：登记脏块指纹作为后续去重基准（writeFrame 同步阻塞，
		// 返回即代表内容进入编码器，无跨协程一致性问题）
		for k, i := range idxs {
			s.vidHash[i] = crcs[k]
		}
		return true, true
	}
	return true, false
}

// pushLosslessFrame 智能无损补帧（与 rdp.session.pushLosslessFrame 同构）：
// 静止期把整帧画面以无损/近无损 tile 推送一次，覆盖视频层对静态文字的
// 4:2:0 压缩模糊；画面恢复变化后由下一视频帧自动覆盖回来。
func (s *vncSession) pushLosslessFrame(snaps *[]tileSnap) {
	s.fbMu.Lock()
	if s.fb == nil {
		s.fbMu.Unlock()
		return
	}
	for i := range s.sentHash {
		s.sentHash[i] = 0
	}
	for i := range s.tiles {
		s.tiles[i] = true
	}
	s.fbMu.Unlock()
	s.vncFbDump("lossless")
	s.collectAndPush(snaps)
	log.Printf("[vnc render] 智能无损：静止补发无损帧")
}

// ===== 调试转储（绿屏取证） =====
//
// exe 同目录存在 vnc_fbdump.flag 标记文件时，把帧缓冲内容写为 PNG
// （fb_resize_*.png / fb_lossless_*.png，按序号循环覆盖），用于对比
// rdpd 侧帧缓冲与浏览器实际显示，判定绿屏来自服务端更新流还是编码/前端。

var fbDumpSeq atomic.Uint32

func (s *vncSession) vncFbDump(tag string) {
	exe, err := os.Executable()
	if err != nil {
		return
	}
	dir := filepath.Dir(exe)
	if _, err := os.Stat(filepath.Join(dir, "vnc_fbdump.flag")); err != nil {
		return
	}
	s.fbMu.Lock()
	if s.fb == nil {
		s.fbMu.Unlock()
		return
	}
	img := image.NewRGBA(image.Rect(0, 0, s.fbW, s.fbH))
	copy(img.Pix, s.fb.Pix)
	s.fbMu.Unlock()
	name := fmt.Sprintf("fb_%s_%d_%d.png", tag, os.Getpid(), fbDumpSeq.Add(1)%16)
	if f, err := os.Create(filepath.Join(dir, name)); err == nil {
		_ = png.Encode(f, img)
		_ = f.Close()
		log.Printf("[vnc debug] fb 转储 %s: %s", tag, name)
	}
}

// onVideoAU 下发一个 H.264 访问单元（一帧的完整码流）
func (s *vncSession) onVideoAU(au []byte) {
	_ = s.w.write("h264", base64.StdEncoding.EncodeToString(au))
}

// clipRect 裁剪矩形到边界内，返回交叠区域（无效时宽或高为 0）
func clipRect(x, y, w, h, bw, bh int) (cx, cy, cw, ch int) {
	cx, cy, cw, ch = x, y, w, h
	if cx < 0 {
		cw += cx
		cx = 0
	}
	if cy < 0 {
		ch += cy
		cy = 0
	}
	if cx+cw > bw {
		cw = bw - cx
	}
	if cy+ch > bh {
		ch = bh - cy
	}
	if cw < 0 {
		cw = 0
	}
	if ch < 0 {
		ch = 0
	}
	return
}

// rawRect Raw 编码矩形：w*h*4 字节（R,G,B,0）
func (s *vncSession) rawRect(r *bufio.Reader, x, y, w, h int) error {
	if w <= 0 || h <= 0 {
		return nil
	}
	if w*h*4 > 64*1024*1024 {
		return errors.New("VNC Raw 矩形过大")
	}
	buf := make([]byte, w*h*4)
	if _, err := io.ReadFull(r, buf); err != nil {
		return err
	}
	s.blitRGBA(buf, w, x, y, w, h)
	return nil
}

// copyRect 帧内块拷贝（快照源区域后再写回，避免自拷贝重叠问题）
func (s *vncSession) copyRect(x, y, w, h, sx, sy int) {
	if w <= 0 || h <= 0 {
		return
	}
	cx, cy, cw, ch := clipRect(x, y, w, h, s.fbW, s.fbH)
	if cw <= 0 || ch <= 0 {
		return
	}
	// 源区域裁剪到帧缓冲范围，同步收缩目标区域
	ssx, ssy := sx+(cx-x), sy+(cy-y)
	if ssx < 0 {
		d := -ssx
		cx += d
		cw -= d
		ssx = 0
	}
	if ssy < 0 {
		d := -ssy
		cy += d
		ch -= d
		ssy = 0
	}
	if ssx+cw > s.fbW {
		cw = s.fbW - ssx
	}
	if ssy+ch > s.fbH {
		ch = s.fbH - ssy
	}
	if cw <= 0 || ch <= 0 {
		return
	}
	tmp := image.NewRGBA(image.Rect(0, 0, cw, ch))
	draw.Draw(tmp, tmp.Bounds(), s.fb, image.Point{ssx, ssy}, draw.Src)
	draw.Draw(s.fb, image.Rect(cx, cy, cx+cw, cy+ch), tmp, image.Point{}, draw.Src)
}

// hextileRect Hextile 编码矩形：16×16 tile 栅格序，bg/fg 跨 tile 保持
func (s *vncSession) hextileRect(r *bufio.Reader, x, y, w, h int) error {
	if w <= 0 || h <= 0 {
		return nil
	}
	tw := (w + 15) / 16
	th := (h + 15) / 16
	var sub [1]byte
	for ty := 0; ty < th; ty++ {
		for tx := 0; tx < tw; tx++ {
			cw, ch := 16, 16
			if rx := w - tx*16; rx < 16 {
				cw = rx
			}
			if ry := h - ty*16; ry < 16 {
				ch = ry
			}
			if _, err := io.ReadFull(r, sub[:]); err != nil {
				return err
			}
			se := sub[0]
			ox, oy := x+tx*16, y+ty*16
			if se&hextileRaw != 0 {
				buf := make([]byte, cw*ch*4)
				if _, err := io.ReadFull(r, buf); err != nil {
					return err
				}
				s.blitRGBA(buf, cw, ox, oy, cw, ch)
				continue
			}
			if se&hextileBgSpecified != 0 {
				if _, err := io.ReadFull(r, s.bg[:]); err != nil {
					return err
				}
			}
			// 背景填充整个 tile
			s.fillRect(ox, oy, cw, ch, s.bg)
			if se&hextileFgSpecified != 0 {
				if _, err := io.ReadFull(r, s.fg[:]); err != nil {
					return err
				}
			}
			if se&hextileAnySubrects != 0 {
				if _, err := io.ReadFull(r, sub[:]); err != nil {
					return err
				}
				colored := se&hextileSubrectsColoured != 0
				for i := 0; i < int(sub[0]); i++ {
					color := s.fg
					var sb [2]byte // x<<4|y, (w-1)<<4|(h-1)
					if colored {
						// 两种格式的像素 padding（alpha/X）字节位置不同：颜色在前时
						// 位于第 4 字节，坐标在前时位于第 6 字节，据此自动判别
						var blk [6]byte
						if _, err := io.ReadFull(r, blk[:]); err != nil {
							return err
						}
						if !s.subrectFmtKnown {
							s.subrectColorFirst = blk[3] == 0xff && blk[5] != 0xff
							s.subrectFmtKnown = true
						}
						var c [4]byte
						if s.subrectColorFirst {
							copy(c[:], blk[0:4])
							sb[0], sb[1] = blk[4], blk[5]
						} else {
							copy(c[:], blk[2:6])
							sb[0], sb[1] = blk[0], blk[1]
						}
						color = c
					} else {
						if _, err := io.ReadFull(r, sb[:]); err != nil {
							return err
						}
					}
					s.fillRect(ox+int(sb[0]>>4), oy+int(sb[0]&0x0F),
						int(sb[1]>>4)+1, int(sb[1]&0x0F)+1, color)
				}
			}
		}
	}
	return nil
}

// blitRGBA 把 R,G,B,0 字节流写入帧缓冲并补 alpha=255（须持有 fbMu）
func (s *vncSession) blitRGBA(data []byte, stride int, x, y, w, h int) {
	cx, cy, cw, ch := clipRect(x, y, w, h, s.fbW, s.fbH)
	if cw <= 0 || ch <= 0 {
		return
	}
	for row := 0; row < ch; row++ {
		src := (row * stride) * 4
		dst := (cy+row)*s.fb.Stride + cx*4
		for col := 0; col < cw; col++ {
			si := src + col*4
			o := dst + col*4
			s.fb.Pix[o] = data[si]
			s.fb.Pix[o+1] = data[si+1]
			s.fb.Pix[o+2] = data[si+2]
			s.fb.Pix[o+3] = 255
		}
	}
}

// fillRect 纯色填充（须持有 fbMu）
func (s *vncSession) fillRect(x, y, w, h int, color [4]byte) {
	cx, cy, cw, ch := clipRect(x, y, w, h, s.fbW, s.fbH)
	if cw <= 0 || ch <= 0 {
		return
	}
	for row := 0; row < ch; row++ {
		dst := (cy+row)*s.fb.Stride + cx*4
		for col := 0; col < cw; col++ {
			o := dst + col*4
			s.fb.Pix[o] = color[0]
			s.fb.Pix[o+1] = color[1]
			s.fb.Pix[o+2] = color[2]
			s.fb.Pix[o+3] = 255
		}
	}
}

// markDirty 标记矩形覆盖的 tile 为脏（须持有 fbMu）
func (s *vncSession) markDirty(x, y, w, h int) {
	cx, cy, cw, ch := clipRect(x, y, w, h, s.fbW, s.fbH)
	if cw <= 0 || ch <= 0 {
		return
	}
	tx0, tx1 := cx/tileW, (cx+cw-1)/tileW
	ty0, ty1 := cy/tileH, (cy+ch-1)/tileH
	for ty := ty0; ty <= ty1; ty++ {
		for tx := tx0; tx <= tx1; tx++ {
			s.tiles[ty*s.tilesX+tx] = true
		}
	}
	// 视频模式：画面已变化，下一渲染帧整帧编码推送
	s.vidDirty.Store(true)
}

// ===== 渲染推送（与 rdp.session 相同的管线） =====

func (s *vncSession) syncLoop() {
	t := time.NewTicker(syncKeepalive)
	defer t.Stop()
	for {
		select {
		case <-s.closed:
			return
		case <-t.C:
			_ = s.w.write("sync", strconv.FormatInt(time.Now().UnixMilli(), 10))
		}
	}
}

func (s *vncSession) renderLoop() {
	t := time.NewTicker(s.renderGap())
	defer t.Stop()
	snaps := make([]tileSnap, 0, 64)
	for {
		select {
		case <-s.closed:
			return
		case <-t.C:
		case <-s.kick:
		}
		// 最小帧间隔限流：帧更新高频到达时避免渲染/编码打满 CPU 与带宽
		if gap := minFrameGap - time.Since(s.lastRender); gap > 0 {
			select {
			case <-time.After(gap):
			case <-s.closed:
				return
			}
		}
		// 视频模式：整帧交 ffmpeg 帧间压缩（仅画面有变化时写入，静止零开销）；
		// 编码管线失效（ffmpeg 缺失/崩溃）时自动回退 tile 管线
		if s.vidWanted {
			ok, changed := s.videoPush()
			if changed {
				s.lastVideoChange = time.Now()
				s.losslessDone = false
			}
			if ok {
				// 智能无损：画面静止 losslessIdleDelay 后补发一帧无损画面，
				// 锐化 H.264 4:2:0 对静态文字/细节的模糊；动态场景不触发（零开销）
				if !changed && !s.losslessDone && time.Since(s.lastVideoChange) >= losslessIdleDelay {
					s.losslessDone = true
					s.pushLosslessFrame(&snaps)
				}
				s.lastRender = time.Now()
				continue
			}
			s.vidWanted = false
			log.Printf("[vnc render] 视频通道失效，已回退 tile 管线")
			// 视频模式按块清脏（内容缓存命中即跳过）：回退后须全量重标脏，
			// 否则已清脏区域永远不会再由 tile 管线推送
			for i := range s.tiles {
				s.tiles[i] = true
			}
		}
		n := s.collectAndPush(&snaps)
		if n < 0 {
			return
		}
		s.lastRender = time.Now()
	}
}

// collectAndPush 收集脏块并编码推送，返回本批块数（-1 表示会话已结束）。
// 编码/批次合并/即时 sync 与 rdp.session 共用同一套管线。
func (s *vncSession) collectAndPush(snaps *[]tileSnap) int {
	// 锁内：收集脏块并拷贝像素快照
	s.fbMu.Lock()
	*snaps = (*snaps)[:0]
	for ty := 0; ty < s.tilesY; ty++ {
		for tx := 0; tx < s.tilesX; tx++ {
			idx := ty*s.tilesX + tx
			if !s.tiles[idx] {
				continue
			}
			ox, oy := tx*tileW, ty*tileH
			tw, th := tileW, tileH
			if ox+tw > s.fbW {
				tw = s.fbW - ox
			}
			if oy+th > s.fbH {
				th = s.fbH - oy
			}
			sub := s.fb.SubImage(image.Rect(ox, oy, ox+tw, oy+th)).(*image.RGBA)
			cp := make([]byte, len(sub.Pix))
			copy(cp, sub.Pix)
			*snaps = append(*snaps, tileSnap{rect: image.Rect(ox, oy, ox+tw, oy+th), pix: cp, stride: sub.Stride, idx: idx})
			s.tiles[idx] = false
		}
	}
	s.fbMu.Unlock()

	if len(*snaps) == 0 {
		return 0
	}
	// 锁外：并行编码（含内容去重）并按块序拼接为一个批次（与 rdp.session 共用管线）
	instrs, nPng, nJpeg, nSkip := encodeTilesParallel(*snaps, s.sentHash)
	var out bytes.Buffer
	for i := range instrs {
		out.WriteString(instrs[i])
	}
	if out.Len() == 0 {
		return 0 // 全部与已推送内容相同，无需发送
	}
	if err := s.w.writeBatch(out.String()); err != nil {
		log.Printf("[vnc render] tile 写出失败: %v", err)
		return -1
	}
	// 批次写完立即 sync：前端 flush 绘制，画面即时呈现
	_ = s.w.write("sync", strconv.FormatInt(time.Now().UnixMilli(), 10))
	n := s.nFrames.Add(1)
	if n%50 == 0 {
		log.Printf("[vnc render] 已推 %d 批（本次 %d 块 %d KB：png=%d jpeg=%d skip=%d）", n, len(*snaps), out.Len()/1024, nPng, nJpeg, nSkip)
	}
	return len(*snaps)
}
