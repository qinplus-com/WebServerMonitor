package main

// MS-RDPECLIP 剪贴板客户端：基于静态虚拟通道 cliprdr。
//  - 文本双向：浏览器 Ctrl+C/V ⇄ 远程桌面剪贴板
//  - 文件单向：远端复制文件（FileGroupDescriptorW + FileContents）→ 浏览器下载
//    （浏览器安全模型拿不到本地系统剪贴板里的文件，本地→远端文件走传输面板上传）
//
// 通道层注意：client→server 每片数据带 channelPduHeader(flags u16 + chunkLen u16，
// 含头，单片 ≤1600 字节)；server→client 同样带该头，大于一片的 PDU 按
// CHANNEL_FLAG_FIRST/LAST 重组。

import (
	"encoding/base64"
	"encoding/binary"
	"log"
	"strings"
	"sync"
	"unicode/utf16"
)

// CLIPRDR 消息类型（MS-RDPECLIP 2.2.5）
const (
	cbMonitorReady        = 0x0001
	cbFormatList          = 0x0002
	cbFormatListResponse  = 0x0003
	cbFormatDataRequest   = 0x0004
	cbFormatDataResponse  = 0x0005
	cbCaps                = 0x0007 // CB_CLIP_CAPS（注意 0x0006 是 CB_TEMP_DIRECTORY）
	cbFileContentsRequest = 0x0008
	cbFileContentsResp    = 0x0009
)

const (
	cbResponseOK = 0x0001 // CB_RESPONSE_OK：请求成功

	// 通道 PDU 头 flags（MS-RDPBCGR 2.2.6.1.1）
	chanFirst = 0x01
	chanLast  = 0x02

	chanChunkLen = 1600 // SVC 单片 payload 上限（client→server 的 length 字段上限）

	cfUnicodeText       = 13   // CF_UNICODETEXT
	fdFileSize          = 0x40 // FILEDESCRIPTOR.dwFlags：大小有效
	fileContentsSize    = 1    // FILECONTENTS dwFlags：取大小
	fileContentsRange   = 2    // FILECONTENTS dwFlags：取范围
	fileChunkSize       = 65536
	fileMaxSize  int64  = 32 << 20 // 单文件上限 32MB（内存中转）
	filesMaxSize int64  = 64 << 20 // 一批总上限
)

// clipFile 一份远端复制到剪贴板的文件
type clipFile struct {
	name string
	size int64
	data []byte
}

// cliprdrChan 一条会话的剪贴板通道
type cliprdrChan struct {
	w    *guacWriter
	send func([]byte) error

	mu       sync.Mutex
	inited   bool           // 已完成 caps/monitor-ready 交换
	announced bool          // 已向服务端宣告本端剪贴板（进入其监视列表）
	text     string         // 浏览器侧文本（本地→远端）
	lastReq  uint32         // 最近发出的 FORMAT_DATA_REQUEST 格式 ID
	fetch    []*clipFile    // 正在取回的文件列表
	fetchIdx int            // 当前文件下标
	fetchPos int64          // 当前文件已取字节
	streamID uint32         // FileContents 流 ID
	done     bool           // 全部取回，等 end 一次性推送
	rx       []byte // 通道层重组缓冲
}

func newCliprdr(w *guacWriter, send func([]byte) error) *cliprdrChan {
	return &cliprdrChan{w: w, send: send}
}

// ===== 通道层 =====

// onData 处理 cliprdr 通道原始数据。
// 每片带 CHANNEL_PDU_HEADER（MS-RDPBCGR 2.2.6.1.1）：length u32（不含头）+ flags u32，
// 大于一片的消息按 CHANNEL_FLAG_FIRST/LAST 重组。
func (c *cliprdrChan) onData(b []byte) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for len(b) >= 8 {
		length := int(binary.LittleEndian.Uint32(b))
		flags := binary.LittleEndian.Uint32(b[4:])
		if length < 0 || 8+length > len(b) {
			log.Printf("cliprdr 通道数据异常 len=%d buf=%d", length, len(b))
			return
		}
		payload := b[8 : 8+length]
		b = b[8+length:]
		switch {
		case flags&chanFirst != 0 && flags&chanLast != 0: // ONLY
			c.dispatch(payload)
		case flags&chanFirst != 0:
			c.rx = append(c.rx[:0], payload...)
		case flags&chanLast != 0:
			c.rx = append(c.rx, payload...)
			c.dispatch(c.rx)
		default: // 中间片
			c.rx = append(c.rx, payload...)
		}
	}
}

// dispatch 解析通道数据流里的 CLIPRDR 消息（一条通道数据可含多条消息）
func (c *cliprdrChan) dispatch(d []byte) {
	for len(d) >= 8 {
		msgType := binary.LittleEndian.Uint16(d)
		msgFlags := binary.LittleEndian.Uint16(d[2:])
		dataLen := int(binary.LittleEndian.Uint32(d[4:]))
		if 8+dataLen > len(d) {
			dataLen = len(d) - 8
		}
		c.handleMsg(msgType, msgFlags, d[8:8+dataLen])
		d = d[8+dataLen:]
	}
}

// ===== 消息处理 =====

func (c *cliprdrChan) handleMsg(msgType, msgFlags uint16, p []byte) {
	switch msgType {
	case cbCaps:
		// 服务端能力协商：回 capabilities + monitor ready（幂等）
		c.init()
	case cbMonitorReady:
		// 服务端通道就绪：回 caps + monitor ready，并宣告本端剪贴板。
		// 客户端必须主动发一次 FormatList 才会进入服务端剪贴板监视列表，
		// 之后服务端剪贴板更新才会推送 CB_FORMAT_LIST 给本端。
		c.init()
		c.announce()
	case cbFormatList:
		c.onFormatList(p)
		c.sendPDU(cbFormatListResponse, cbResponseOK, nil)
	case cbFormatListResponse:
		// 本端 FormatList 已被服务端接收
	case cbFormatDataRequest:
		c.onFormatDataRequest(binary.LittleEndian.Uint32(p))
	case cbFormatDataResponse:
		if msgFlags&cbResponseOK == 0 {
			c.abortFetch()
			return
		}
		c.onFormatDataResponse(p)
	case cbFileContentsResp:
		if msgFlags&cbResponseOK == 0 {
			c.abortFetch()
			return
		}
		c.onFileContentsResponse(p)
	case cbFileContentsRequest:
		// 本端从不宣告文件格式，服务端不应请求；忽略
	}
}

// init 发送 capabilities + monitor ready（仅一次）
func (c *cliprdrChan) init() {
	if c.inited {
		return
	}
	c.inited = true
	log.Printf("cliprdr 通道就绪")
	// GeneralCapabilitySet：capSetType=1, capSetLen=12, version=2, generalFlags=0
	caps := make([]byte, 12)
	binary.LittleEndian.PutUint16(caps[0:], 1)
	binary.LittleEndian.PutUint16(caps[2:], 12)
	binary.LittleEndian.PutUint32(caps[4:], 2)
	c.sendPDU(cbCaps, 0, caps)
	c.sendPDU(cbMonitorReady, 0, nil)
}

// announce 向服务端宣告本端剪贴板状态（幂等）。
// 浏览器侧已有文本时宣告 CF_UNICODETEXT，否则宣告空列表。
func (c *cliprdrChan) announce() {
	if c.announced {
		return
	}
	c.announced = true
	if c.text != "" {
		c.sendPDU(cbFormatList, 0, []byte{13, 0})
	} else {
		c.sendPDU(cbFormatList, 0, nil)
	}
}

// onFormatList 解析服务端剪贴板格式列表：优先文件（FileGroupDescriptorW），其次文本
func (c *cliprdrChan) onFormatList(p []byte) {
	off := 0
	var fileFmt uint32
	hasFiles, hasText := false, false
	for off+2 <= len(p) {
		id := uint32(binary.LittleEndian.Uint16(p[off:]))
		off += 2
		if id >= 0xC000 {
			// 注册格式：szFormatName（ASCII，NUL 结尾，无长度前缀）
			end := off
			for end < len(p) && p[end] != 0 {
				end++
			}
			name := string(p[off:end])
			off = end
			if off < len(p) {
				off++ // 跳过 NUL
			}
			if strings.EqualFold(name, "FileGroupDescriptorW") {
				fileFmt = id
				hasFiles = true
			}
		} else if id == cfUnicodeText {
			hasText = true
		}
	}
	switch {
	case hasFiles:
		c.lastReq = fileFmt
		c.requestFormat(fileFmt)
	case hasText:
		c.lastReq = cfUnicodeText
		c.requestFormat(cfUnicodeText)
	}
}

// onFormatDataRequest 服务端要本端剪贴板内容（本地→远端文本）
func (c *cliprdrChan) onFormatDataRequest(formatID uint32) {
	if formatID == cfUnicodeText && c.text != "" {
		c.sendPDU(cbFormatDataResponse, cbResponseOK, utf16Encode(c.text))
	} else {
		c.sendPDU(cbFormatDataResponse, 0, nil) // CB_RESPONSE_FAIL
	}
}

// onFormatDataResponse 服务端返回格式数据（文件描述符表 或 文本）
func (c *cliprdrChan) onFormatDataResponse(p []byte) {
	if c.lastReq == cfUnicodeText {
		// 远端文本 → 浏览器剪贴板（guac clipboard 流）
		c.emitClipboard(utf16Decode(p))
		return
	}
	if c.lastReq >= 0xC000 && len(c.fetch) == 0 {
		// FileGroupDescriptorW：cItems + FILEDESCRIPTORW[552 字节/个]
		if len(p) < 4 {
			return
		}
		items := int(binary.LittleEndian.Uint32(p))
		off := 4
		var total int64
		for i := 0; i < items && off+552 <= len(p); i++ {
			desc := p[off : off+552]
			off += 552
			var size int64
			if binary.LittleEndian.Uint32(desc)&fdFileSize != 0 {
				size = int64(binary.LittleEndian.Uint32(desc[28:]))<<32 |
					int64(binary.LittleEndian.Uint32(desc[24:]))
			} else {
				size = -1 // 未带大小，先取 FILECONTENTS_SIZE
			}
			name := strings.TrimRight(utf16Decode(desc[32:552]), "\x00")
			if name == "" || size > fileMaxSize || total+max64(size, 0) > filesMaxSize {
				continue // 超限跳过
			}
			total += size
			c.fetch = append(c.fetch, &clipFile{name: name, size: size})
		}
		if len(c.fetch) > 0 {
			log.Printf("cliprdr 远端复制 %d 个文件", len(c.fetch))
			c.fetchIdx = 0
			c.fetchPos = 0
			c.fetchOne()
		}
	}
}

// onFileContentsResponse 服务端返回文件内容分片（streamId u32 + data）
func (c *cliprdrChan) onFileContentsResponse(p []byte) {
	if len(p) < 4 || c.fetchIdx >= len(c.fetch) {
		return
	}
	f := c.fetch[c.fetchIdx]
	data := p[4:]

	if f.size < 0 {
		// 首个 SIZE 响应：8 字节 u64
		if len(data) >= 8 {
			f.size = int64(binary.LittleEndian.Uint64(data))
		}
		if f.size <= 0 || f.size > fileMaxSize {
			c.nextFile()
			return
		}
		f.data = make([]byte, 0, f.size)
		c.fetchPos = 0
		c.requestRange()
		return
	}

	f.data = append(f.data, data...)
	c.fetchPos += int64(len(data))
	if len(data) == 0 || c.fetchPos >= f.size {
		c.nextFile()
		return
	}
	c.requestRange()
}

// fetchOne 开始取第 fetchIdx 个文件
func (c *cliprdrChan) fetchOne() {
	if c.fetchIdx >= len(c.fetch) {
		c.pushFiles()
		return
	}
	f := c.fetch[c.fetchIdx]
	if f.size == 0 { // 空文件
		f.data = []byte{}
		c.nextFile()
		return
	}
	if f.size < 0 {
		// 请求 FILECONTENTS_SIZE（lIndex 当前文件）
		c.sendReq(fileContentsSize, 0, 8)
		return
	}
	f.data = make([]byte, 0, f.size)
	c.requestRange()
}

// requestRange 请求当前文件 [fetchPos, fetchPos+64KB) 内容
func (c *cliprdrChan) requestRange() {
	n := int64(fileChunkSize)
	if rest := c.fetch[c.fetchIdx].size - c.fetchPos; rest < n {
		n = rest
	}
	c.sendReq(fileContentsRange, c.fetchPos, uint32(n))
}

func (c *cliprdrChan) sendReq(dwFlags uint32, pos int64, cb uint32) {
	c.streamID++
	// CLIPRDR_FILECONTENTS_REQUEST: streamId, lIndex, dwFlags, nPosition(u64), cbRequested
	p := make([]byte, 24)
	binary.LittleEndian.PutUint32(p[0:], c.streamID)
	binary.LittleEndian.PutUint32(p[4:], uint32(c.fetchIdx))
	binary.LittleEndian.PutUint32(p[8:], dwFlags)
	binary.LittleEndian.PutUint64(p[12:], uint64(pos))
	binary.LittleEndian.PutUint32(p[20:], cb)
	c.sendPDU(cbFileContentsRequest, 0, p)
}

func (c *cliprdrChan) nextFile() {
	c.fetchIdx++
	c.fetchPos = 0
	c.fetchOne()
}

func (c *cliprdrChan) abortFetch() {
	c.fetch = nil
	c.fetchIdx = 0
}

// requestFormat 请求服务端某格式的剪贴板数据
func (c *cliprdrChan) requestFormat(formatID uint32) {
	p := make([]byte, 4)
	binary.LittleEndian.PutUint32(p, formatID)
	c.sendPDU(cbFormatDataRequest, 0, p)
}

// emitClipboard 以 guac clipboard 流推送远端文本
func (c *cliprdrChan) emitClipboard(text string) {
	emitGuacClipboard(c.w, text)
}

// emitGuacClipboard 以 guac clipboard 流推送文本（RDP/VNC 共用）
func emitGuacClipboard(w *guacWriter, text string) {
	_ = w.write("clipboard", "0", "text/plain")
	for len(text) > 0 {
		// 按 UTF-8 字符边界切块
		n := 4000
		if len(text) < n {
			n = len(text)
		}
		cut := text[:n]
		for n < len(text) && text[n]&0xC0 == 0x80 {
			cut += string(text[n])
			n++
		}
		text = text[n:]
		_ = w.write("blob", "0", base64.StdEncoding.EncodeToString([]byte(cut)))
	}
	_ = w.write("end", "0")
}

// pushFiles 文件全部取回 → 以 guac file 流推给浏览器（前端缓冲后确认下载）
func (c *cliprdrChan) pushFiles() {
	for _, f := range c.fetch {
		log.Printf("cliprdr 推送文件 %s (%d 字节)", f.name, len(f.data))
		_ = c.w.write("file", "0", "application/octet-stream", f.name)
		for off := 0; off < len(f.data); off += 6048 {
			end := off + 6048
			if end > len(f.data) {
				end = len(f.data)
			}
			_ = c.w.write("blob", "0", base64.StdEncoding.EncodeToString(f.data[off:end]))
		}
		_ = c.w.write("end", "0")
	}
	c.abortFetch()
}

// ===== 本端动作（浏览器方向） =====

// setLocalText 浏览器复制了文本 → 向服务端宣告剪贴板内容
func (c *cliprdrChan) setLocalText(text string) {
	if text == "" {
		return
	}
	c.mu.Lock()
	c.text = text
	c.announced = true
	c.init()
	// CF_UNICODETEXT 标准格式：2 字节 formatId，无名称
	c.sendPDU(cbFormatList, 0, []byte{13, 0})
	c.mu.Unlock()
}

// ===== 发送 =====

// sendPDU 组 CLIPRDR 头并分片经通道发送。
// 每片 = CHANNEL_PDU_HEADER（length u32 不含头 + flags u32）+ payload，payload ≤1600。
func (c *cliprdrChan) sendPDU(msgType, msgFlags uint16, payload []byte) {
	pdu_ := make([]byte, 8+len(payload))
	binary.LittleEndian.PutUint16(pdu_[0:], msgType)
	binary.LittleEndian.PutUint16(pdu_[2:], msgFlags)
	binary.LittleEndian.PutUint32(pdu_[4:], uint32(len(payload)))
	copy(pdu_[8:], payload)

	first := true
	for len(pdu_) > 0 {
		n := chanChunkLen
		if len(pdu_) < n {
			n = len(pdu_)
		}
		chunk := pdu_[:n]
		pdu_ = pdu_[n:]
		last := len(pdu_) == 0

		var flags uint32
		if first && last {
			flags = chanFirst | chanLast
		} else if first {
			flags = chanFirst
		} else if last {
			flags = chanLast
		}
		first = false

		hdr := make([]byte, 8)
		binary.LittleEndian.PutUint32(hdr, uint32(n))
		binary.LittleEndian.PutUint32(hdr[4:], flags)
		_ = c.send(append(hdr, chunk...))
	}
}

// ===== 工具 =====

func utf16Encode(s string) []byte {
	u := utf16.Encode([]rune(s))
	b := make([]byte, 2*len(u))
	for i, v := range u {
		binary.LittleEndian.PutUint16(b[2*i:], v)
	}
	return b
}

func utf16Decode(b []byte) string {
	if len(b)%2 != 0 {
		b = b[:len(b)-1]
	}
	u := make([]uint16, len(b)/2)
	for i := range u {
		u[i] = binary.LittleEndian.Uint16(b[2*i:])
	}
	return string(utf16.Decode(u))
}

func max64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}
