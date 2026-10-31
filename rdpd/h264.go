package main

// H.264 视频通道：帧缓冲整帧写入 ffmpeg（libx264 zerolatency）编码为 Annex-B 码流，
// 按访问单元（AU=一帧）拆分后以 h264 指令下发；前端 WebCodecs 解码直绘默认层画布。
//
// 与 tile 管线的关系：握手参数 codec=h264 且 ffmpeg 可用时启用，renderLoop 走
// 整帧编码分支（帧间压缩，全屏连续变化场景带宽降低一个数量级）；ffmpeg 缺失或
// 运行出错时自动回退 tile 管线，功能不受损。不引入 cgo，仅依赖 ffmpeg 可执行
// 文件（与 rdpd 同目录或 PATH 中）。

import (
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"sync"
	"sync/atomic"
	"time"
)

// videoPipe 一个会话的 H.264 编码管线（ffmpeg 子进程管理 + 码流按帧拆分）
type videoPipe struct {
	mu     sync.Mutex
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	onAU   func([]byte)
	gen    uint64 // 编码器世代号：换代后旧进程读出侧停止下发，防止尾部 AU 与新码流交错（绿屏/花屏）
	frames atomic.Int64 // 已写入编码器的帧数
	bytes  atomic.Int64 // 已输出的码流字节数
	// 编码延迟观测（近似）：writeFrame 落笔 → 该帧 AU 到达读出侧的墙钟差。
	// 编码管线前后帧轻度重叠时略偏高，用于分辨率/画质档位的延迟横向对比
	lastWriteNs atomic.Int64
	latN        atomic.Int64
	latSumUs    atomic.Int64
	latMaxUs    atomic.Int64
	segW, segH  int // 当前编码段分辨率（stop 时随统计一并输出）
}

// evenAlign 向上取偶：yuv420p 编码要求宽高为偶数，帧缓冲多出的至多 1px 为黑边不可见
func evenAlign(v int) int {
	if v%2 != 0 {
		return v + 1
	}
	return v
}

// ffmpegPath 定位 ffmpeg：优先 rdpd 同目录（部署目录），其次 PATH
func ffmpegPath() string {
	name := "ffmpeg"
	if runtime.GOOS == "windows" {
		name = "ffmpeg.exe"
	}
	if exe, err := os.Executable(); err == nil {
		p := filepath.Join(filepath.Dir(exe), name)
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	if p, err := exec.LookPath("ffmpeg"); err == nil {
		return p
	}
	return ""
}

// start 启动（或重启）编码器。w/h 为帧缓冲尺寸（须为偶数，yuv420p 要求），
// fps 为目标帧率（IDR 周期按 5 秒换算），crf 为画质档位（流畅25/超清21/原画17）。
// 新进程首帧输出必为 IDR：分辨率变更 / 前端 keyframe 请求通过再次 start
// 重建进程实现强制关键帧。
func (vp *videoPipe) start(w, h, fps, crf int, onAU func([]byte)) error {
	vp.stop()
	if w <= 0 || h <= 0 || w%2 != 0 || h%2 != 0 {
		return fmt.Errorf("分辨率 %dx%d 无法编码 yuv420p（须为正偶数）", w, h)
	}
	if fps <= 0 {
		fps = 30
	}
	path := ffmpegPath()
	if path == "" {
		return errors.New("未找到 ffmpeg（需与 rdpd 部署在同一目录或 PATH 中）")
	}
	cmd := exec.Command(path,
		"-hide_banner", "-loglevel", "error",
		// 输入：RGBA 原始帧（与帧缓冲内存布局一致，整帧直写）
		"-f", "rawvideo", "-pixel_format", "rgba",
		"-video_size", fmt.Sprintf("%dx%d", w, h), "-framerate", strconv.Itoa(fps),
		"-i", "pipe:0",
		// x264 zerolatency：无 lookahead/B帧/sync-lookahead + 分片多线程，端到端延迟≈1帧；
		// baseline profile 最大化浏览器（含硬解）兼容性——ultrafast 本就关闭 CABAC/8x8dct，
		// 转 baseline 无画质损失；-g 每 5 秒强制 IDR 兜底自愈丢包/异常；
		// -flush_packets 1 逐包写出压低编码器到读出侧的缓冲延迟。
		// libx264 私有参数统一经 -x264-params 传入（跨 ffmpeg 版本兼容，
		// 各参数与 zerolatency tune 内置行为一致，显式声明防 tune 语义变化）
		"-vf", "format=yuv420p",
		"-c:v", "libx264", "-preset", "ultrafast", "-tune", "zerolatency",
		"-profile:v", "baseline",
		"-crf", strconv.Itoa(crf), "-g", strconv.Itoa(fps*5),
		"-x264-params", "sliced-threads=1:rc-lookahead=0:sync-lookahead=0",
		"-flush_packets", "1",
		"-f", "h264", "pipe:1",
	)
	cmd.Stderr = os.Stderr // 诊断进 rdpd 日志（网关重定向 rdpd 的 stderr）
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return err
	}

	vp.mu.Lock()
	vp.gen++ // 换代：旧读出侧据此停止下发尾部 AU
	gen := vp.gen
	vp.cmd = cmd
	vp.stdin = stdin
	vp.onAU = onAU
	vp.segW, vp.segH = w, h
	vp.mu.Unlock()
	vp.frames.Store(0)
	vp.bytes.Store(0)
	vp.latN.Store(0)
	vp.latSumUs.Store(0)
	vp.latMaxUs.Store(0)

	// 读出 goroutine：解析 Annex-B 码流，按 AU 回调下发；进程退出（EOF）后
	// 清空句柄使 writeFrame 返回错误，触发会话自动回退 tile 管线。
	// 回调前校验世代号：start 被 stop+重启后，旧进程 Kill 前残留在 stdout
	// 缓冲中的 AU 不得下发（与新编码器的 IDR 交错会导致前端参考链错乱）
	go func() {
		sp := &auSplitter{}
		buf := make([]byte, 128*1024)
		for {
			n, rerr := stdout.Read(buf)
			if n > 0 {
				for _, au := range sp.push(buf[:n]) {
					vp.mu.Lock()
					current := vp.gen == gen
					cb := vp.onAU
					vp.mu.Unlock()
					if current && cb != nil {
						if t0 := vp.lastWriteNs.Load(); t0 > 0 {
							if us := (time.Now().UnixNano() - t0) / 1000; us > 0 {
								vp.latN.Add(1)
								vp.latSumUs.Add(us)
								for {
									m := vp.latMaxUs.Load()
									if us <= m || vp.latMaxUs.CompareAndSwap(m, us) {
										break
									}
								}
							}
						}
						vp.bytes.Add(int64(len(au)))
						cb(au)
					}
				}
			}
			if rerr != nil {
				break
			}
		}
		vp.mu.Lock()
		if vp.cmd == cmd {
			vp.cmd, vp.stdin = nil, nil
		}
		vp.mu.Unlock()
		_ = cmd.Wait() // 回收子进程（Kill/EOF 后很快返回）
	}()
	return nil
}

// stop 停止编码器（幂等）：stdin EOF + Kill 双保险尽快退出，回收由读出侧完成
func (vp *videoPipe) stop() {
	// 换代/会话结束前输出上一编码段统计（分辨率切换/会话结束即出数），随后清零
	if n := vp.frames.Load(); n > 0 {
		vp.mu.Lock()
		w, h := vp.segW, vp.segH
		vp.mu.Unlock()
		avgMs, maxMs := 0.0, 0.0
		if ln := vp.latN.Load(); ln > 0 {
			avgMs = float64(vp.latSumUs.Load()) / float64(ln) / 1000
			maxMs = float64(vp.latMaxUs.Load()) / 1000
		}
		log.Printf("[video] 编码段统计 %dx%d：%d 帧，%d KB，编码延迟 avg=%.1fms max=%.1fms",
			w, h, n, vp.bytes.Load()/1024, avgMs, maxMs)
		vp.frames.Store(0)
		vp.bytes.Store(0)
		vp.latN.Store(0)
		vp.latSumUs.Store(0)
		vp.latMaxUs.Store(0)
	}
	vp.mu.Lock()
	cmd, stdin := vp.cmd, vp.stdin
	vp.cmd, vp.stdin = nil, nil
	vp.mu.Unlock()
	if stdin != nil {
		_ = stdin.Close()
	}
	if cmd != nil && cmd.Process != nil {
		_ = cmd.Process.Kill()
	}
}

// active 编码器是否在运行
func (vp *videoPipe) active() bool {
	vp.mu.Lock()
	defer vp.mu.Unlock()
	return vp.cmd != nil
}

// writeFrame 写入一帧 RGBA 像素（尺寸须与 start 一致）。ffmpeg 消费不及
// 时 Write 阻塞（背压），由读出 goroutine 持续消费自然解除。
func (vp *videoPipe) writeFrame(pix []byte) error {
	vp.mu.Lock()
	stdin := vp.stdin
	vp.mu.Unlock()
	if stdin == nil {
		return errors.New("H.264 编码器未运行")
	}
	vp.lastWriteNs.Store(time.Now().UnixNano())
	if _, err := stdin.Write(pix); err != nil {
		return err
	}
	if n := vp.frames.Add(1); n%30 == 0 {
		// 每 30 帧输出吞吐与编码延迟观测（latency 为 writeFrame→AU 回调的近似墙钟差）
		avgMs, maxMs := 0.0, 0.0
		if ln := vp.latN.Load(); ln > 0 {
			avgMs = float64(vp.latSumUs.Load()) / float64(ln) / 1000
			maxMs = float64(vp.latMaxUs.Load()) / 1000
		}
		log.Printf("[video] 已编码 %d 帧，输出 %d KB，编码延迟 avg=%.1fms max=%.1fms", n, vp.bytes.Load()/1024, avgMs, maxMs)
	}
	return nil
}

// ===== Annex-B 码流按访问单元拆分 =====

// auSplitter 把 ffmpeg 输出的连续 Annex-B 码流按访问单元（一帧）拆分。
// AU 边界判定：新 VCL NAL 且 first_mb_in_slice == 0——zerolatency 的
// sliced-threads 使一帧输出多个 slice NAL（IDR 帧各 slice 同为 type 5），
// 仅凭 NAL 类型无法判帧，须解析 slice 头部首个 ue(v)；SPS/PPS/SEI 等
// 非 VCL NAL 归属于随后 VCL NAL 所在帧。
type auSplitter struct {
	buf     []byte // 接收缓冲（含当前未完 AU）
	auStart int    // 当前 AU 在 buf 中的起始偏移
	scan    int    // 下一次 start code 搜索起点
	haveVCL bool   // 当前 AU 是否已含 VCL NAL
}

// push 喂入新数据，返回其中完整拆出的 AU（各 AU 为独立拷贝）
func (sp *auSplitter) push(data []byte) [][]byte {
	sp.buf = append(sp.buf, data...)
	var aus [][]byte
	for {
		p, l := findStartCode(sp.buf, sp.scan)
		// start code 不存在，或其后的 NAL 头字节尚未到齐：等下一批数据
		if p < 0 || p+l+1 > len(sp.buf) {
			break
		}
		t := int(sp.buf[p+l] & 0x1F)
		if t >= 1 && t <= 5 && sp.haveVCL && firstMBInSlice(sp.buf[p+l:]) == 0 {
			au := make([]byte, p-sp.auStart)
			copy(au, sp.buf[sp.auStart:p])
			aus = append(aus, au)
			sp.auStart = p
			sp.haveVCL = false
		}
		if t >= 1 && t <= 5 {
			sp.haveVCL = true
		}
		sp.scan = p + l
	}
	// 丢弃已发出的前缀，保留当前 AU 起点之后的数据
	if sp.auStart > 0 {
		n := copy(sp.buf, sp.buf[sp.auStart:])
		sp.buf = sp.buf[:n]
		sp.scan -= sp.auStart
		sp.auStart = 0
	}
	return aus
}

// findStartCode 从 from 起查找 Annex-B start code（3/4 字节形式），返回位置与长度
func findStartCode(b []byte, from int) (int, int) {
	for i := from; i+2 < len(b); i++ {
		if b[i] == 0 && b[i+1] == 0 && b[i+2] == 1 {
			if i > 0 && b[i-1] == 0 {
				return i - 1, 4 // 4 字节形式（x264 固定使用）
			}
			return i, 3
		}
	}
	return -1, 0
}

// firstMBInSlice 解析 slice NAL 头部的 first_mb_in_slice ue(v)。
// 头部可能含 emulation prevention 字节（00 00 03→00 00），解析前去模拟；
// 数据不足时返回 -1（保守视作同帧 slice，倾向合并——解码器可自行消化
// 单 chunk 多 AU，而误拆分会产生坏帧）。
func firstMBInSlice(nal []byte) int {
	rbsp := make([]byte, 0, 6)
	zeros := 0
	for i := 1; i < len(nal) && len(rbsp) < 6; i++ { // 跳过 1 字节 NAL 头
		b := nal[i]
		if zeros == 2 && b == 3 {
			zeros = 0
			continue
		}
		if b == 0 {
			zeros++
		} else {
			zeros = 0
		}
		rbsp = append(rbsp, b)
	}
	if len(rbsp) < 3 {
		return -1
	}
	// rbsp[0] 起即切片头首字节（NAL 头已在收集时跳过）：first_mb_in_slice 的
	// ue(v) 从第 0 位开始。此前误置 8 会再跳过一个字节，吞掉 first_mb 高 8 位，
	// 同帧后续 slice 被误判为新帧边界 → AU 拆出残帧（顶部一 slice 正确、
	// 其余宏块未初始化解码为绿色）——绿屏且按分辨率确定性复现的根因
	bitPos := 0
	bit := func() int {
		if bitPos>>3 >= len(rbsp) {
			return 1 // 越界按 1 处理：终止前导零计数，倾向"非边界"
		}
		v := (rbsp[bitPos>>3] >> (7 - uint(bitPos&7))) & 1
		bitPos++
		return int(v)
	}
	z := 0
	for bit() == 0 && z < 32 {
		z++
	}
	v := 0
	for i := 0; i < z; i++ {
		v = v<<1 | bit()
	}
	return (1 << z) - 1 + v
}
