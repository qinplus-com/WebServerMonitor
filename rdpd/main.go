package main

// rdpd —— 内置 RDP 引擎（guacd 兼容协议的轻量替代）。
//
// 网关在 config.yaml 未配置 guacd_addr（空或 "-"）时自动拉起本进程，
// 通过 stdin/stdout 以 Guacamole 协议通信（协议角色等同 guacd）：
//
//	网关 -> rdpd : select / size / image / audio / video / args / 输入指令
//	rdpd -> 网关 : args / ready / size / png / jpeg / cursor / sync / error
//
// stdout 只承载协议指令；诊断日志一律走 stderr。
//
// 许可：GPL-3.0（依赖 github.com/x90skysn3k/grdp）。

import (
	"bufio"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"strconv"

	"github.com/x90skysn3k/grdp/glog"
)

// connParams 从 guacd 握手中提取的连接参数
type connParams struct {
	proto    string // rdp / vnc（select 指令携带）
	host     string
	port     int
	username string
	password string
	width    int
	height   int
	codec    string // h264 = 前端请求视频通道（WebCodecs）；空 = 传统 tile 管线
	quality  string // 画质档位：fluent=流畅(CRF25) / normal=超清(CRF21) / high=原画(CRF17)
	fps      int    // 渲染帧率上限（30/60），0=默认 30
}

// crfForQuality 画质档位 → x264 CRF。对应商用远控的画质分档思路
// （ToDesk 流畅/高清/超清/原画、UU 码率分档）：把画质/带宽取舍权交给用户。
func crfForQuality(q string) int {
	switch q {
	case "fluent":
		return 25
	case "high":
		return 17
	default:
		return 21
	}
}

// fpsForSession 渲染帧率上限（异常值回落 30）
func fpsForSession(fps int) int {
	if fps != 30 && fps != 60 {
		return 30
	}
	return fps
}

// engineSession 内置引擎会话（RDP / VNC）统一接口
type engineSession interface {
	run() error
	stop()
	handleInstruction(op string, args []string)
}

func main() {
	listen := flag.String("listen", "", "调试模式：监听地址（如 127.0.0.1:4822），默认走 stdio")
	debug := flag.Bool("debug", false, "输出 grdp 协议调试日志（stderr）")
	flag.Parse()

	glog.SetLogger(log.New(os.Stderr, "", 0))
	if *debug {
		glog.SetLevel(glog.DEBUG)
	} else {
		glog.SetLevel(glog.NONE)
	}

	if *listen != "" {
		serveListen(*listen)
		return
	}
	if err := runStdio(os.Stdin, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "rdpd:", err)
		os.Exit(1)
	}
}

// serveListen 调试模式：按 guacd 方式监听 TCP
func serveListen(addr string) {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		fmt.Fprintln(os.Stderr, "rdpd listen:", err)
		os.Exit(1)
	}
	log.Printf("rdpd 调试模式监听 %s", addr)
	for {
		c, err := ln.Accept()
		if err != nil {
			return
		}
		go func() {
			defer c.Close()
			if err := runStream(c, c); err != nil {
				log.Printf("会话结束: %v", err)
			}
		}()
	}
}

// runStdio stdio 模式（网关拉起）
func runStdio(in io.Reader, out io.WriteCloser) error {
	return runStream(in, out)
}

func runStream(in io.Reader, out io.WriteCloser) error {
	r := bufio.NewReaderSize(in, 64*1024)
	w := newGuacWriter(out)

	// ===== guacd 角色握手 =====
	p, mimes, err := handshake(r, w)
	if err != nil {
		return fmt.Errorf("握手失败: %w", err)
	}

	// ===== 启动远程会话 =====
	var s engineSession
	if p.proto == "vnc" {
		s = newVNCSession(p, w, mimes)
	} else {
		s = newSession(p, w, mimes)
	}
	go func() {
		if err := s.run(); err != nil {
			fmt.Fprintln(os.Stderr, "rdpd 会话结束:", err)
		}
		s.stop()
		// RDP 已断开：结束进程（stdout 随之关闭，网关侧感知并断开 WebSocket）
		os.Exit(0)
	}()

	// ===== 输入指令循环 =====
	for {
		op, args, err := guacReadInstruction(r)
		if err != nil {
			// EOF 为对端正常关闭；其余视为协议错误，记日志便于排查
			if err != io.EOF {
				log.Printf("rdpd 输入解析结束: %v", err)
			}
			s.stop()
			return nil // 对端关闭
		}
		if op == "disconnect" {
			s.stop()
			return nil
		}
		s.handleInstruction(op, args)
	}
}

// handshake 扮演 guacd 完成 select/args/size/image/args → ready 握手
func handshake(r *bufio.Reader, w *guacWriter) (connParams, []string, error) {
	var p connParams

	op, args, err := guacReadInstruction(r)
	if err != nil {
		return p, nil, err
	}
	if op != "select" {
		return p, nil, fmt.Errorf("期望 select，收到 %q", op)
	}
	if len(args) > 0 && args[0] != "rdp" && args[0] != "vnc" {
		_ = w.write("error", "内置引擎仅支持 RDP/VNC 协议", "UNSUPPORTED")
		return p, nil, fmt.Errorf("不支持的协议: %v", args)
	}
	if len(args) > 0 {
		p.proto = args[0]
	}

	argNames := []string{"hostname", "port", "username", "password",
		"security", "ignore-cert", "resize-method", "client-name", "codec", "quality", "fps"}
	reply := append([]string{"args"}, argNames...)
	if err := w.write(reply...); err != nil {
		return p, nil, err
	}

	// 收集 size / image / args（audio/video 忽略）
	var mimes []string
	var values []string
	for {
		op, args, err := guacReadInstruction(r)
		if err != nil {
			return p, nil, err
		}
		switch op {
		case "size":
			if len(args) >= 2 {
				p.width, _ = strconv.Atoi(args[0])
				p.height, _ = strconv.Atoi(args[1])
			}
		case "image":
			mimes = args
		case "args":
			values = args
			goto done
		case "audio", "video":
			// 不支持，忽略
		default:
			// 容错忽略其余指令
		}
	}
done:
	if p.width <= 0 {
		p.width = 1024
	}
	if p.height <= 0 {
		p.height = 768
	}
	for i, name := range argNames {
		if i >= len(values) {
			break
		}
		switch name {
		case "hostname":
			p.host = values[i]
		case "port":
			p.port, _ = strconv.Atoi(values[i])
		case "username":
			p.username = values[i]
		case "password":
			p.password = values[i]
		case "codec":
			p.codec = values[i]
		case "quality":
			p.quality = values[i]
		case "fps":
			p.fps, _ = strconv.Atoi(values[i])
		}
	}
	p.fps = fpsForSession(p.fps)
	if p.port <= 0 {
		if p.proto == "vnc" {
			p.port = 5900
		} else {
			p.port = 3389
		}
	}
	if p.host == "" {
		return p, nil, errors.New("缺少 hostname 参数")
	}

	// ready（携带随机会话 id）
	id := make([]byte, 8)
	_, _ = rand.Read(id)
	if err := w.write("ready", hex.EncodeToString(id)); err != nil {
		return p, nil, err
	}
	return p, mimes, nil
}
