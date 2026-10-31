package main

// RDP 会话：使用 grdp 公开 API 建立 RDP 连接，
// 位图更新 → Guacamole png/jpeg 指令；Guacamole 输入指令 → RDP 输入事件。

import (
	"bytes"
	"encoding/base64"
	"errors"
	"hash/crc32"
	"image"
	"image/draw"
	"image/jpeg"
	"image/png"
	"log"
	"net"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/x90skysn3k/grdp/core"
	"github.com/x90skysn3k/grdp/protocol/nla"
	"github.com/x90skysn3k/grdp/protocol/pdu"
	"github.com/x90skysn3k/grdp/protocol/sec"
	"github.com/x90skysn3k/grdp/protocol/t125"
	"github.com/x90skysn3k/grdp/protocol/t125/gcc"
	"github.com/x90skysn3k/grdp/protocol/tpkt"
	"github.com/x90skysn3k/grdp/protocol/x224"
)

const (
	dialTimeout    = 10 * time.Second
	sessionTimeout = 25 * time.Second // RDP 会话建立（含 NLA 认证）总时限
	syncKeepalive  = time.Second      // 空闲期 sync 保活（每帧 batch 后立即随帧发 sync）

	// 渲染节奏：renderLoop 以 1/fps 上限 ticker 驱动（fps=握手参数，30/60）+
	// 位图/帧更新到达即时唤醒（最小帧间隔限流）。输入 → 画面显示的典型延迟
	// 为 15~33ms（60fps 档 15~16ms），接近本地操作手感。
	minFrameGap = 15 * time.Millisecond

	// tile 尺寸：小块贴合常见小更新（光标闪烁/文字输入等），减少无效像素
	// 的编码与传输；全帧重绘时块数增多但单块编码极快，总耗时反而更低
	tileW = 160
	tileH = 120

	// 智能无损：视频模式下画面静止该时长后，补发一帧无损 tile 画面，
	// 消除 H.264 4:2:0 色度抽样对静态文字/细节的模糊（对标 ToDesk「智能无损」）
	losslessIdleDelay = 3 * time.Second
)

// pngFast 无损 PNG 编码器：BestSpeed 相比默认压缩级快 3~5 倍，
// 对文本/扁平块（PNG 的适用场景）体积几乎不变
var pngFast = png.Encoder{CompressionLevel: png.BestSpeed}

// encodeTile 编码一个脏块为 Guacamole 图像指令，返回指令与编码类型（png/jpeg）。
// 自适应编码（与 guacd 同思路）：先编无损 PNG，体积超过 1 字节/像素
// （照片/渐变类内容，PNG 体积爆炸）时改用有损 JPEG q90（4:4:4 无色度抽样）；
// 文本/扁平 UI 保持 PNG 像素级无损。JPEG 体积通常仅为 PNG 的 1/5~1/10。
func encodeTile(sp *tileSnap) (string, string) {
	img := &image.RGBA{Pix: sp.pix, Stride: sp.stride, Rect: sp.rect}
	var buf bytes.Buffer
	_ = pngFast.Encode(&buf, img)
	op, data := "png", buf.Bytes()
	if buf.Len() > img.Rect.Dx()*img.Rect.Dy() {
		var jbuf bytes.Buffer
		if jpeg.Encode(&jbuf, img, &jpeg.Options{Quality: 90}) == nil && jbuf.Len() < buf.Len() {
			op, data = "jpeg", jbuf.Bytes()
		}
	}
	// mask 必须用 0：前端将 12(copy) 解析为"清空整个画布再绘制"，
	// 局部块用它会导致画面整体清空黑屏；0 保持 source-over 增量叠加
	return guacEncode(op, "0", "0",
		strconv.Itoa(sp.rect.Min.X), strconv.Itoa(sp.rect.Min.Y),
		base64.StdEncoding.EncodeToString(data)), op
}

// guacamole 鼠标掩码
const (
	guacLeft   = 1
	guacMiddle = 2
	guacRight  = 4
	guacWUp    = 8
	guacWDown  = 16
)

type session struct {
	params   connParams
	w        *guacWriter
	supports map[string]bool // 客户端支持的图像 mimetypes

	pc        *pdu.Client
	x         *x224.X224 // 传输栈根，用于会话结束/超时时关闭底层连接
	live      bool
	lastMask  int
	closed    chan struct{}
	closeOnce sync.Once

	// 流媒体模式：RDP 增量位图先合成进完整帧缓冲，
	// 由 renderLoop 按固定帧率推送（MJPEG 式），
	// 从结构上避免增量块缝隙/闪烁问题。
	// 推送以 tile 为单位：仅编码发送有变化的块，脏块过多时退化为整帧，
	// 静止画面几乎零流量，避免整帧推送打满带宽导致浏览器断开。
	fb      *image.RGBA
	fbMu    sync.Mutex
	fbW     int
	fbH     int
	tilesX  int
	tilesY  int
	tiles   []bool // 各 tile 是否有未推送的变化
	// sentHash 记录各 tile 最近一次成功推送内容的 CRC32C 校验值（0=从未发送），
	// 编码前比对可跳过与已发送内容相同的块（光标闪烁/时钟/服务端重复重绘）
	sentHash []uint64
	nFrames  atomic.Int64

	// 渲染唤醒：位图到达时即时触发 renderLoop（限流见 minFrameGap），
	// 不必等下一个 33ms tick，压低输入→画面延迟
	kick       chan struct{}
	lastRender time.Time

	// 会话真实分辨率探测：RDP 协商不回服务端桌面尺寸（ServerCoreData 无此字段），
	// 帧缓冲只能按请求值初始化。Windows 对已有会话的重新附加不一定应用请求分辨率，
	// 实际画面可能大于请求（位图越界→扩容）或小于请求（位图不触边→超时收缩）。
	readyAt       time.Time
	shrinkChecked bool
	seenMaxW      int
	seenMaxH      int

	// 剪贴板通道与浏览器 clipboard 流缓冲（clipboard → blob* → end）
	clip        *cliprdrChan
	clipStreams map[int][]byte

	// H.264 视频通道（h264.go）：codec=h264 且 ffmpeg 可用时启用，renderLoop
	// 整帧交 ffmpeg 帧间压缩；失效自动回退 tile 管线。vidRestart 置位（分辨率
	// 变更/前端 keyframe 请求）时下一渲染帧重建编码器以强制 IDR。
	vid        videoPipe
	vidWanted  bool
	vidDirty   atomic.Bool
	vidRestart atomic.Bool
	// vidHash 各 tile 最近一次"已实际编码"内容的 CRC32C 指纹（内容缓存）：
	// 脏块与该指纹一致时视为画面未变，直接清脏跳过整帧编码。仅在 renderLoop
	// 协程内读写；编码器重建时整体作废（新编码器不持有任何参考内容）。
	vidHash []uint64

	// 智能无损状态（仅 renderLoop 协程读写）：lastVideoChange 最近一次
	// 实际编码视频帧的时刻；losslessDone 当前静止期是否已补发无损帧
	lastVideoChange time.Time
	losslessDone    bool
}

func newSession(p connParams, w *guacWriter, mimes []string) *session {
	s := &session{
		params:   p,
		w:        w,
		supports: map[string]bool{},
		closed:   make(chan struct{}),
		kick:     make(chan struct{}, 1),
	}
	for _, m := range mimes {
		s.supports[m] = true
	}
	return s
}

func (s *session) stop() {
	s.closeOnce.Do(func() {
		close(s.closed)
		s.vid.stop() // 停止 H.264 编码管线（幂等）
		if s.x != nil {
			_ = s.x.Close() // 幂等：连接阶段失败时 grdp 已自行关闭
		}
	})
}

func (s *session) fail(msg string) {
	if msg != "" {
		_ = s.w.write("error", msg, "SERVER_ERROR")
	}
	s.stop()
}

// run 建立 RDP 连接并阻塞至会话结束。
// 返回错误 = 连接阶段失败（已通过 fail 下发 error 指令）。
func (s *session) run() error {
	defer s.stop()

	ready := make(chan struct{}, 1)
	pc, err := s.connect(ready)
	if err != nil {
		s.fail("无法连接远程桌面：" + err.Error())
		return err
	}
	s.pc = pc

	select {
	case <-ready:
	case <-time.After(sessionTimeout):
		err = errors.New("RDP 会话建立超时，请确认远程主机已开启远程桌面且凭据正确")
		s.fail(err.Error())
		return err
	case <-s.closed:
		return errors.New("已取消")
	}

	// 会话就绪：下发远端尺寸与默认光标，启动流媒体渲染与 sync 刷新
	s.live = true
	s.readyAt = time.Now()
	log.Printf("rdpd 会话就绪 %s:%d 请求 %dx%d", s.params.host, s.params.port, s.params.width, s.params.height)
	// 帧缓冲宽高向上取偶：H.264 视频通道要求 yuv420p 偶数尺寸（至多 1px 黑边不可见）
	s.fbW, s.fbH = evenAlign(s.params.width), evenAlign(s.params.height)
	s.fb = image.NewRGBA(image.Rect(0, 0, s.fbW, s.fbH))
	s.tilesX = (s.fbW + tileW - 1) / tileW
	s.tilesY = (s.fbH + tileH - 1) / tileH
	s.tiles = make([]bool, s.tilesX*s.tilesY)
	s.sentHash = make([]uint64, s.tilesX*s.tilesY)
	s.vidHash = make([]uint64, s.tilesX*s.tilesY)
	_ = s.w.write("size", "0", strconv.Itoa(s.fbW), strconv.Itoa(s.fbH))
	_ = s.w.write("size", "-1", strconv.Itoa(cursorW), strconv.Itoa(cursorH))
	_ = s.w.write("png", "12", "-1", "0", "0", cursorPNGBase64)
	_ = s.w.write("cursor", "0", "0", "-1", "0", "0", strconv.Itoa(cursorW), strconv.Itoa(cursorH))
	// 视频模式初始化：前端请求 codec=h264 且 ffmpeg 可用时启用；失败自动回退 tile。
	// 置脏使首个渲染帧立即推送 IDR，前端尽早完成解码器配置（SPS/PPS 先于画面到达）
	if s.params.codec == "h264" {
		if err := s.vid.start(s.fbW, s.fbH, s.params.fps, crfForQuality(s.params.quality), s.onVideoAU); err == nil {
			s.vidWanted = true
			s.vidDirty.Store(true)
			s.lastVideoChange = time.Now()
			log.Printf("[video] 视频通道已启用（libx264 %dx%d %dfps crf%d %s）",
				s.fbW, s.fbH, s.params.fps, crfForQuality(s.params.quality), s.params.quality)
		} else {
			log.Printf("[video] 视频通道不可用，使用 tile 管线: %v", err)
		}
	}
	go s.renderLoop()

	ticker := time.NewTicker(syncKeepalive)
	defer ticker.Stop()
	for {
		select {
		case <-s.closed:
			return nil
		case <-ticker.C:
			_ = s.w.write("sync", strconv.FormatInt(time.Now().UnixMilli(), 10))
		}
	}
}

// connect 拨号并完成 RDP 协议握手（NLA 优先）。
// 注意：grdp 的握手是异步回调链（x224.Connect 仅发出请求即返回），
// 且其 ctx 机制会把 deadline 写死到整条 TCP 连接上（SocketLayer.SetContext），
// 与异步握手/长会话均冲突，故这里不传 ctx——建立超时由 run() 的
// time.After(sessionTimeout) 兜底，超时/结束时经 stop() 关闭传输栈。
func (s *session) connect(ready chan struct{}) (*pdu.Client, error) {
	// Windows RDP 会话分辨率下限为 1024×768：请求低于该值会被系统强制提升，
	// 造成会话实际分辨率与请求不符（前端按请求建帧缓冲导致比例失配/黑边）。
	// 这里主动钳制到下限，配合前端等比缩放显示。
	if s.params.width < 1024 {
		s.params.width = 1024
	}
	if s.params.height < 768 {
		s.params.height = 768
	}
	addr := net.JoinHostPort(s.params.host, strconv.Itoa(s.params.port))
	conn, err := net.DialTimeout("tcp", addr, dialTimeout)
	if err != nil {
		return nil, errors.New("无法连接 " + addr + "：" + err.Error())
	}

	domain, user := splitUser(s.params.username)
	sock := core.NewSocketLayer(conn) // TLS 默认忽略服务端证书
	tp := tpkt.New(sock, nla.NewNTLMv2(domain, user, s.params.password))
	x := x224.New(tp)
	mcs := t125.NewMCSClient(x)
	secC := sec.NewClient(mcs)
	pc := pdu.NewClient(secC)
	s.x = x

	// 客户端性能标志（随 Client Info PDU 下发）：关闭全窗拖拽/菜单动画/
	// 主题/光标阴影等纯装饰特效，服务端少推大量位图流量；保留字体平滑保证
	// 文字可读性。壁纸不再关闭：桌面背景一次性绘制后即静止，H.264 帧间
	// 压缩下增量成本可忽略（tile 管线回退时也仅多一次全屏 PNG 推送）。
	secC.SetPerformanceFlags(sec.PERF_DISABLE_FULLWINDOWDRAG |
		sec.PERF_DISABLE_MENUANIMATIONS | sec.PERF_DISABLE_THEMING | sec.PERF_DISABLE_CURSOR_SHADOW |
		sec.PERF_ENABLE_FONT_SMOOTHING)

	mcs.SetClientDesktop(uint16(s.params.width), uint16(s.params.height))
	secC.SetUser(user)
	secC.SetPwd(s.params.password)
	secC.SetDomain(domain)
	tp.SetFastPathListener(secC)
	secC.SetFastPathListener(pc)
	x.SetRequestedProtocol(x224.PROTOCOL_SSL | x224.PROTOCOL_HYBRID)

	// 剪贴板（MS-RDPECLIP）：注册 cliprdr 静态虚拟通道并路由收发
	mcs.ClientNetworkData().AddVirtualChannel("cliprdr",
		uint32(gcc.CHANNEL_OPTION_INITIALIZED)|gcc.CHANNEL_OPTION_COMPRESS_RDP)
	secC.SetChannelSender(mcs)
	s.clipStreams = map[int][]byte{}
	s.clip = newCliprdr(s.w, func(b []byte) error {
		_, err := secC.SendToChannel("cliprdr", b)
		return err
	})
	secC.On("channel", func(name string, data []byte) {
		if name == "cliprdr" {
			s.clip.onData(data)
		}
	})

	pc.On("ready", func() { ready <- struct{}{} })
	pc.On("bitmap", s.onBitmap)
	pc.On("error", func(e error) { s.fail("远程桌面会话错误：" + e.Error()) })
	pc.On("close", func() { s.stop() })
	// NLA/TLS 协商发生在 x224 层，认证失败（如密码错误）从这里冒出
	x.On("error", func(e error) {
		msg := e.Error()
		if strings.Contains(msg, "status") || strings.Contains(msg, "CredSSP") || strings.Contains(msg, "NTLM") {
			s.fail("认证失败（NLA/CredSSP）：" + msg)
			return
		}
		s.fail("连接协商失败：" + msg)
	})

	if err := x.Connect(); err != nil {
		return nil, errors.New(err.Error())
	}
	return pc, nil
}

// splitUser 拆分 DOMAIN\user
func splitUser(user string) (domain, name string) {
	if i := strings.LastIndexAny(user, `\/`); i >= 0 {
		return user[:i], user[i+1:]
	}
	return "", user
}

// ===== 输入 =====

func (s *session) sendPointer(flags int, x, y int) {
	if !s.live {
		return
	}
	pe := &pdu.PointerEvent{PointerFlags: uint16(flags), XPos: uint16(x), YPos: uint16(y)}
	s.pc.SendInputEvents(pdu.INPUT_EVENT_MOUSE, []pdu.InputEventsInterface{pe})
}

func (s *session) sendKey(def keyDef, down bool) {
	if !s.live {
		return
	}
	flags := 0
	if def.ext {
		flags |= pdu.KBDFLAGS_EXTENDED
	}
	if !down {
		flags |= pdu.KBDFLAGS_RELEASE
	}
	ke := &pdu.ScancodeKeyEvent{KeyboardFlags: uint16(flags), KeyCode: uint16(def.sc)}
	s.pc.SendInputEvents(pdu.INPUT_EVENT_SCANCODE, []pdu.InputEventsInterface{ke})
}

// handleInstruction 处理浏览器方向的一条 Guacamole 指令
func (s *session) handleInstruction(op string, args []string) {
	atoi := func(i int) int {
		if i < len(args) {
			v, _ := strconv.Atoi(args[i])
			return v
		}
		return 0
	}
	switch op {
	case "mouse":
		x, y, mask := atoi(0), atoi(1), atoi(2)
		s.handleMouse(x, y, mask)
	case "key":
		def, ok := keysymMap[atoi(0)]
		if ok {
			s.sendKey(def, atoi(1) != 0)
		}
	case "disconnect":
		s.stop()
	case "clipboard":
		// 浏览器剪贴板流开始（后续 blob*/end 携带内容）
		if s.clipStreams == nil {
			s.clipStreams = map[int][]byte{}
		}
		s.clipStreams[atoi(0)] = []byte{}
	case "blob":
		if b, ok := s.clipStreams[atoi(0)]; ok && len(args) > 1 {
			if raw, err := base64.StdEncoding.DecodeString(args[1]); err == nil {
				s.clipStreams[atoi(0)] = append(b, raw...)
			}
		}
	case "end":
		if b, ok := s.clipStreams[atoi(0)]; ok {
			delete(s.clipStreams, atoi(0))
			if s.clip != nil {
				go s.clip.setLocalText(string(b))
			}
		}
	case "keyframe":
		// 前端解码异常/标签页重聚焦：重建编码器强制 IDR，重建解码参考链
		if s.vidWanted {
			s.vidRestart.Store(true)
			s.vidDirty.Store(true)
		}
	default:
		// nop / ack / sync / size（一期不支持动态分辨率）等，忽略
	}
}

func (s *session) handleMouse(x, y, mask int) {
	if !s.live {
		return
	}
	// 移动（事件本身携带坐标，按钮/滚轮事件同理）
	s.sendPointer(int(pdu.PTRFLAGS_MOVE), x, y)

	// 滚轮：每次事件一格（±120），旋转值编码于 flag 低 9 位
	if mask&guacWUp != 0 {
		s.sendPointer(int(pdu.PTRFLAGS_WHEEL)|1, x, y)
	}
	if mask&guacWDown != 0 {
		s.sendPointer(int(pdu.PTRFLAGS_WHEEL)|int(pdu.PTRFLAGS_WHEEL_NEGATIVE)|1, x, y)
	}

	// 按钮状态变化
	buttons := mask & (guacLeft | guacMiddle | guacRight)
	prev := s.lastMask & (guacLeft | guacMiddle | guacRight)
	if buttons != prev {
		trans := []struct {
			bit  int
			flag uint16
		}{
			{guacLeft, pdu.PTRFLAGS_BUTTON1},
			{guacMiddle, pdu.PTRFLAGS_BUTTON3},
			{guacRight, pdu.PTRFLAGS_BUTTON2},
		}
		for _, t := range trans {
			now := buttons&t.bit != 0
			was := prev&t.bit != 0
			if now == was {
				continue
			}
			flags := int(t.flag)
			if now {
				flags |= int(pdu.PTRFLAGS_DOWN)
			}
			s.sendPointer(flags, x, y)
		}
	}
	s.lastMask = mask
}

// ===== 位图 → 帧缓冲 → 整帧推送 =====

// onBitmap 把 RDP 增量位图合成进帧缓冲（不逐块下发）
func (s *session) onBitmap(rects []pdu.BitmapData) {
	s.fbMu.Lock()
	defer s.fbMu.Unlock()
	if s.fb == nil {
		return
	}
	// 帧缓冲按"请求的分辨率"初始化，但 Windows 对已有会话的重新附加
	// 不一定应用新分辨率（会话保持其历史尺寸）。位图一旦超出当前帧缓冲，
	// 说明真实会话更大：扩容帧缓冲到 tile 对齐尺寸，并通知前端同步画布。
	needW, needH := 0, 0
	for i := range rects {
		r := &rects[i]
		if rw := int(r.DestLeft) + int(r.Width); rw > needW {
			needW = rw
		}
		if rh := int(r.DestTop) + int(r.Height); rh > needH {
			needH = rh
		}
	}
	if needW > s.fbW || needH > s.fbH {
		newW := ((needW + tileW - 1) / tileW) * tileW
		newH := ((needH + tileH - 1) / tileH) * tileH
		newFB := image.NewRGBA(image.Rect(0, 0, newW, newH))
		draw.Draw(newFB, image.Rect(0, 0, s.fbW, s.fbH), s.fb, image.Point{}, draw.Src)
		s.fb = newFB
		s.fbW, s.fbH = newW, newH
		s.tilesX = (s.fbW + tileW - 1) / tileW
		s.tilesY = (s.fbH + tileH - 1) / tileH
		s.tiles = make([]bool, s.tilesX*s.tilesY)
		s.sentHash = make([]uint64, s.tilesX*s.tilesY)
		s.vidHash = make([]uint64, s.tilesX*s.tilesY)
		for i := range s.tiles {
			s.tiles[i] = true // 全量重发，覆盖新旧区域
		}
		log.Printf("rdpd 会话实际分辨率 %dx%d（请求 %dx%d），已同步帧缓冲", s.fbW, s.fbH, s.params.width, s.params.height)
		s.w.write("size", "0", strconv.Itoa(s.fbW), strconv.Itoa(s.fbH))
		// 视频模式：分辨率变化须重建编码器（新尺寸 + 强制 IDR）；补一次 sync
		// 让前端立即 flush 画布 resize（否则最大延迟一个 sync 周期才重绘）
		if s.vidWanted {
			s.vidRestart.Store(true)
		}
		_ = s.w.write("sync", strconv.FormatInt(time.Now().UnixMilli(), 10))
	}
	for i := range rects {
		r := &rects[i]
		w, h := int(r.Width), int(r.Height)
		if w <= 0 || h <= 0 || w > 8192 || h > 8192 {
			continue
		}
		bpp := int(r.BitsPerPixel) / 8
		data := r.BitmapDataStream
		if r.IsCompress() {
			data = core.Decompress(data, w, h, bpp)
		}
		if len(data) < w*h*bpp || bpp < 2 {
			continue
		}
		x0, y0 := int(r.DestLeft), int(r.DestTop)
		// 记录位图累计覆盖边界（会话真实分辨率的唯一信号来源）
		if x0+w > s.seenMaxW {
			s.seenMaxW = x0 + w
		}
		if y0+h > s.seenMaxH {
			s.seenMaxH = y0 + h
		}
		// 裁剪到帧缓冲范围
		if x0 < 0 || y0 < 0 || x0 >= s.fbW || y0 >= s.fbH {
			continue
		}
		if x0+w > s.fbW {
			w = s.fbW - x0
		}
		if y0+h > s.fbH {
			h = s.fbH - y0
		}
		// grdp 的 Decompress 输出已是自顶向下的标准布局
		for row := 0; row < h; row++ {
			src := (row * w) * bpp
			dst := (y0+row)*s.fb.Stride + x0*4
			for col := 0; col < w; col++ {
				b, g, red := pixelRGB(data, src+col*bpp, bpp)
				o := dst + col*4
				s.fb.Pix[o], s.fb.Pix[o+1], s.fb.Pix[o+2], s.fb.Pix[o+3] = red, g, b, 255
			}
		}
		// 标记该矩形覆盖的 tile 为脏
		tx0, tx1 := x0/tileW, (x0+w-1)/tileW
		ty0, ty1 := y0/tileH, (y0+h-1)/tileH
		for ty := ty0; ty <= ty1; ty++ {
			for tx := tx0; tx <= tx1; tx++ {
				s.tiles[ty*s.tilesX+tx] = true
			}
		}
	}
	// 视频模式：位图已到达，下一渲染帧整帧编码推送
	s.vidDirty.Store(true)
	// 位图已到达：唤醒渲染循环立即推送（渲染侧按 minFrameGap 限流）
	select {
	case s.kick <- struct{}{}:
	default:
	}
}

// maybeShrinkLocked 会话就绪 3 秒后做一次收缩检查（需持有 fbMu）。
// 位图从未触及帧缓冲的右/下边界（覆盖 <85%）说明真实会话分辨率小于请求
// （Windows 重新附加已有会话时可能维持历史尺寸）：收缩帧缓冲并通知前端，
// 避免画面右/下出现永远填不满的黑区。误判可自愈——之后位图越界会触发扩容。
func (s *session) maybeShrinkLocked() {
	if s.shrinkChecked || s.fb == nil {
		return
	}
	if time.Since(s.readyAt) < 3*time.Second {
		return
	}
	s.shrinkChecked = true
	if s.seenMaxW == 0 || s.seenMaxH == 0 {
		return // 会话未输出过画面，等扩容逻辑兜底
	}
	if s.seenMaxW >= s.fbW*85/100 && s.seenMaxH >= s.fbH*85/100 {
		return // 覆盖充分，视为真实分辨率与请求一致
	}
	newW := ((s.seenMaxW + tileW - 1) / tileW) * tileW
	newH := ((s.seenMaxH + tileH - 1) / tileH) * tileH
	if newW >= s.fbW && newH >= s.fbH {
		return
	}
	newFB := image.NewRGBA(image.Rect(0, 0, newW, newH))
	draw.Draw(newFB, image.Rect(0, 0, newW, newH), s.fb, image.Point{}, draw.Src)
	s.fb = newFB
	s.fbW, s.fbH = newW, newH
	s.tilesX = (s.fbW + tileW - 1) / tileW
	s.tilesY = (s.fbH + tileH - 1) / tileH
	s.tiles = make([]bool, s.tilesX*s.tilesY)
	s.sentHash = make([]uint64, s.tilesX*s.tilesY)
	s.vidHash = make([]uint64, s.tilesX*s.tilesY)
	for i := range s.tiles {
		s.tiles[i] = true
	}
	log.Printf("rdpd 会话实际分辨率 %dx%d（请求 %dx%d），已收缩帧缓冲", s.fbW, s.fbH, s.params.width, s.params.height)
	s.w.write("size", "0", strconv.Itoa(s.fbW), strconv.Itoa(s.fbH))
	// 视频模式：分辨率变化须重建编码器（新尺寸 + 强制 IDR）；补 sync 同上
	if s.vidWanted {
		s.vidRestart.Store(true)
	}
	s.vidDirty.Store(true)
	_ = s.w.write("sync", strconv.FormatInt(time.Now().UnixMilli(), 10))
}

// renderGap 渲染帧间隔（1/fps，帧率上限由握手参数 fps 决定，30/60）
func (s *session) renderGap() time.Duration {
	return time.Second / time.Duration(fpsForSession(s.params.fps))
}

// videoPush 视频模式渲染步：必要时重建编码器（分辨率变更/关键帧请求），
// 画面有变化时把整帧 RGBA 写入 ffmpeg。返回 (ok, changed)：ok=false 表示
// 编码管线失效（回退 tile）；changed=true 表示本帧实际写入了编码器。
// 写入前先做脏块内容去重（vidHash 缓存）：与上次已编码帧一致的块直接清脏，
// 光标闪烁/时钟跳动等无变化重绘不再触发整帧拷贝与编码。
func (s *session) videoPush() (bool, bool) {
	s.fbMu.Lock()
	s.maybeShrinkLocked()
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
			crcs = append(crcs, uint64(c))
		}
		if !any {
			dirty = false
		} else {
			pix = make([]byte, len(s.fb.Pix))
			copy(pix, s.fb.Pix) // 锁内拷贝快照，锁外写入，避免长时间持锁阻塞位图合成
		}
	}
	// 重建编码器时同步抓整帧快照与全帧指纹：keyframe 请求常发生在静止画面
	// （tiles 已全部被 vidHash 清脏），若仅置 vidDirty，下一帧会因无脏块而
	// 吞掉 IDR，前端（decoder 重建/画布 resize 后）将永久等不到关键帧直至
	// 画面再次变化——绿屏/冻结的直接成因。这里锁内备好快照，锁外强推。
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
		// 编码器换代：内容缓存整体作废（新编码器尚未编码过任何内容，
		// 置零防误命中；vidHash 仅 renderLoop 协程读写，无需加锁）
		for i := range s.vidHash {
			s.vidHash[i] = 0
		}
		if err := s.vid.start(w, h, s.params.fps, crfForQuality(s.params.quality), s.onVideoAU); err != nil {
			log.Printf("[video] 编码器重建失败: %v", err)
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
			s.vidDirty.Store(true) // 会话极早期（fb 未就绪）：等首个位图驱动
		}
		return true, false
	}
	if dirty && !s.vid.active() {
		return false, false // 编码器已退出（读出侧 EOF 清空句柄）：立即回退 tile 管线
	}
	if dirty {
		if err := s.vid.writeFrame(pix); err != nil {
			log.Printf("[video] 帧写入失败: %v", err)
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

// pushLosslessFrame 智能无损补帧：把当前整帧画面以无损/近无损 tile
// （PNG 自适应 JPEG，均为 4:4:4 色度）推送一次，覆盖视频层对静态文字/细节
// 的 4:2:0 压缩模糊。画面恢复变化后由下一视频帧自动覆盖回来。
// sentHash 清零强制重发全部块（无视去重缓存）；vidHash 不动——视频层
// 的内容基准不受补帧影响。
func (s *session) pushLosslessFrame(snaps *[]tileSnap) {
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
	s.collectAndPush(snaps)
	log.Printf("[render] 智能无损：静止补发无损帧")
}

// onVideoAU 下发一个 H.264 访问单元（一帧的完整码流）
func (s *session) onVideoAU(au []byte) {
	_ = s.w.write("h264", base64.StdEncoding.EncodeToString(au))
}

// tileSnap 一个脏块的像素快照（锁内拷贝、锁外编码，避免长时间持锁阻塞位图写入）
type tileSnap struct {
	rect   image.Rectangle
	pix    []byte
	stride int
	idx    int // 在脏块网格中的下标（用于 sentHash 去重比对）
}

// renderLoop 把帧缓冲增量推给前端：
// 33ms ticker（30fps 上限）+ 位图到达即时唤醒，仅编码发送有变化的 tile；
// 一次 tick 的全部指令经 writeBatch 合并为单次写入，随后立即下发 sync——
// 网关并入单个 WebSocket 帧，浏览器同帧完成绘制并即时 flush，
// 消除分块闪烁与等待 sync 周期的额外延迟。
func (s *session) renderLoop() {
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
		// 最小帧间隔限流：位图高频到达时避免渲染/编码打满 CPU 与带宽
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
			log.Printf("[render] 视频通道失效，已回退 tile 管线")
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

// castagnoliTable CRC32C 硬件加速校验表（SSE4.2，~10GB/s，去重比对开销可忽略）
var castagnoliTable = crc32.MakeTable(crc32.Castagnoli)

// tileCRC 计算一个脏块区域的 CRC32C 内容指纹（逐行链式更新）。
// 视频模式的整帧去重（vidHash 缓存）与 tile 管线的块去重共用。
func tileCRC(fb *image.RGBA, tilesX, i int) uint64 {
	tx := (i % tilesX) * tileW
	ty := (i / tilesX) * tileH
	w, h := tileW, tileH
	if tx+w > fb.Rect.Dx() {
		w = fb.Rect.Dx() - tx
	}
	if ty+h > fb.Rect.Dy() {
		h = fb.Rect.Dy() - ty
	}
	if w <= 0 || h <= 0 {
		return 0
	}
	crc := uint32(0)
	for row := 0; row < h; row++ {
		off := (ty+row)*fb.Stride + tx*4
		crc = crc32.Update(crc, castagnoliTable, fb.Pix[off:off+w*4])
	}
	return uint64(crc)
}

// encodeTilesParallel 并行编码脏块并按原顺序返回指令列表（跳过的块返回空串）。
// 全帧重绘时 100+ 块串行编码（PNG+JPEG 兜底）可达数百毫秒，直接拉高
// 拖拽/滚动场景的帧时；按 CPU 核数并行可将编码时间摊到各核，块序不变。
// 编码前先做内容去重：与该块上次已推送内容（CRC32C）一致时直接跳过，
// 光标闪烁/时钟跳动/服务端重复重绘等场景不再产生任何编码与传输。
func encodeTilesParallel(snaps []tileSnap, sentHash []uint64) (instrs []string, nPng, nJpeg, nSkip int) {
	instrs = make([]string, len(snaps))
	encode := func(sp *tileSnap) (string, string, bool) {
		if sentHash != nil && sp.idx < len(sentHash) {
			h := uint64(crc32.Checksum(sp.pix, castagnoliTable))
			if h == 0 {
				h = 1 // 避免与"从未发送"的 0 值哨兵冲突（全零内容也强制非 0）
			}
			if sentHash[sp.idx] == h {
				return "", "", true
			}
			// 并行 worker 写入互不相同的下标，无需原子操作
			sentHash[sp.idx] = h
		}
		instr, codec := encodeTile(sp)
		return instr, codec, false
	}
	if len(snaps) >= 4 && runtime.NumCPU() > 1 {
		workers := runtime.NumCPU()
		if workers > 8 {
			workers = 8
		}
		var (
			wg      sync.WaitGroup
			next    atomic.Int64
			nPng64  atomic.Int64
			nJpeg64 atomic.Int64
			nSkip64 atomic.Int64
		)
		for w := 0; w < workers; w++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for {
					i := int(next.Add(1)) - 1
					if i >= len(snaps) {
						return
					}
					instr, codec, skip := encode(&snaps[i])
					instrs[i] = instr
					if skip {
						nSkip64.Add(1)
					} else if codec == "jpeg" {
						nJpeg64.Add(1)
					} else {
						nPng64.Add(1)
					}
				}
			}()
		}
		wg.Wait()
		return instrs, int(nPng64.Load()), int(nJpeg64.Load()), int(nSkip64.Load())
	}
	for i := range snaps {
		instr, codec, skip := encode(&snaps[i])
		instrs[i] = instr
		if skip {
			nSkip++
		} else if codec == "jpeg" {
			nJpeg++
		} else {
			nPng++
		}
	}
	return instrs, nPng, nJpeg, nSkip
}

// collectAndPush 收集脏块并编码推送，返回本批块数（-1 表示会话已结束）。
func (s *session) collectAndPush(snaps *[]tileSnap) int {
	// 锁内：收集脏块并拷贝像素快照
	s.fbMu.Lock()
	s.maybeShrinkLocked()
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
	// 锁外：并行编码（含内容去重）并按块序拼接为一个批次
	instrs, nPng, nJpeg, nSkip := encodeTilesParallel(*snaps, s.sentHash)
	var out bytes.Buffer
	for i := range instrs {
		out.WriteString(instrs[i])
	}
	if out.Len() == 0 {
		return 0 // 全部与已推送内容相同，无需发送
	}
	if err := s.w.writeBatch(out.String()); err != nil {
		log.Printf("[render] tile 写出失败: %v", err)
		return -1
	}
	// 批次写完立即 sync：前端 flush 绘制，画面即时呈现
	_ = s.w.write("sync", strconv.FormatInt(time.Now().UnixMilli(), 10))
	n := s.nFrames.Add(1)
	if n%50 == 0 {
		log.Printf("[render] 已推 %d 批（本次 %d 块 %d KB：png=%d jpeg=%d skip=%d）", n, len(*snaps), out.Len()/1024, nPng, nJpeg, nSkip)
	}
	return len(*snaps)
}

func pixelRGB(data []byte, off, bpp int) (b, g, r byte) {
	switch bpp {
	case 4:
		return data[off], data[off+1], data[off+2]
	case 3:
		return data[off], data[off+1], data[off+2]
	default: // 16bpp RGB565 小端
		v := uint16(data[off]) | uint16(data[off+1])<<8
		r5 := byte((v >> 11) & 0x1F)
		g6 := byte((v >> 5) & 0x3F)
		b5 := byte(v & 0x1F)
		return (b5 << 3) | (b5 >> 2), (g6 << 2) | (g6 >> 4), (r5 << 3) | (r5 >> 2)
	}
}
