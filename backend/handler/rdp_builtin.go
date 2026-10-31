package handler

// 内置 H5 远程桌面引擎：config.yaml 未配置 guacd_addr（空或 "-"）时启用。
//
// 网关拉起同目录的 rdpd 辅助进程（guacd 兼容协议，stdio 通信），
// 本文件仅做进程生命周期管理 + Guacamole 指令双向桥接，
// RDP 协议实现全部位于 rdpd（独立 GPL-3.0 组件，见 rdpd/ 目录）。

import (
	"bufio"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"

	"github.com/gorilla/websocket"
)

func rdpdBuiltinName() string {
	if runtime.GOOS == "windows" {
		return "rdpd.exe"
	}
	return "rdpd"
}

// builtinRDPSession 启动 rdpd 并把浏览器 WebSocket 与之桥接（复用 guacd 握手逻辑）。
// codec 为前端请求的视频编码通道（h264 = WebCodecs）；quality/fps 为画质档位
// 与渲染帧率上限，透传给 rdpd。
func builtinRDPSession(conn *websocket.Conn, info rdpTokenInfo, width, height int, codec, quality string, fps int) {
	exe, err := os.Executable()
	if err != nil {
		guacFail(conn, "内置远程桌面引擎启动失败："+err.Error())
		return
	}
	bin := filepath.Join(filepath.Dir(exe), rdpdBuiltinName())
	if _, err := os.Stat(bin); err != nil {
		guacFail(conn, "未找到内置远程桌面引擎（"+rdpdBuiltinName()+" 需与网关程序部署在同一目录）")
		return
	}

	cmd := exec.Command(bin)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		guacFail(conn, "内置远程桌面引擎启动失败："+err.Error())
		return
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		guacFail(conn, "内置远程桌面引擎启动失败："+err.Error())
		return
	}
	// rdpd 的诊断日志走 stderr，写入 exe 同目录 rdpd.log 便于排查（服务模式下 stderr 无处可去）
	if f, err := os.OpenFile(filepath.Join(filepath.Dir(exe), "rdpd.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644); err == nil {
		cmd.Stderr = f
	} else {
		cmd.Stderr = os.Stderr
	}

	if err := cmd.Start(); err != nil {
		guacFail(conn, "内置远程桌面引擎启动失败："+err.Error())
		return
	}
	defer func() {
		_ = stdin.Close()
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	}()

	reader := bufio.NewReaderSize(stdout, 64*1024)
	if err := guacHandshake(reader, stdin, nil, info, width, height, codec, quality, fps); err != nil {
		guacSendError(conn, "远程桌面连接失败："+err.Error())
		return
	}

	log.Printf("H5 远程桌面(内置引擎)已连接：server=%d user=%d %s:%d", info.ServerID, info.UserID, info.IP, info.Port)

	done := make(chan struct{}, 2)
	var fwdFrames, fwdBytes int64
	// rdpd -> 浏览器（按 Guacamole 指令边界重组；rdpd 每个 tick 一次性写出
	// 全部绘图指令，此处把缓冲区内连续的完整指令合并为单个 WebSocket 帧，
	// 前端在同一渲染帧内完成全部绘制，消除逐块刷出的闪烁感）
	go func() {
		var batch []byte
		flush := func() bool {
			if len(batch) == 0 {
				return true
			}
			if werr := conn.WriteMessage(websocket.TextMessage, batch); werr != nil {
				log.Printf("H5 远程桌面(内置引擎) ws写出失败: %v（已转发 %d 帧）", werr, fwdFrames)
				return false
			}
			fwdFrames++
			fwdBytes += int64(len(batch))
			batch = batch[:0]
			return true
		}
		for {
			line, err := reader.ReadBytes(';')
			if len(line) > 0 {
				batch = append(batch, line...)
				// 缓冲区已无连续数据：本批为一个绘制单元，合并发送
				if reader.Buffered() == 0 && !flush() {
					break
				}
			}
			if err != nil {
				_ = flush()
				break
			}
		}
		done <- struct{}{}
	}()
	// 浏览器 -> rdpd
	var brInstr int64
	go func() {
		for {
			_, data, err := conn.ReadMessage()
			if err != nil {
				break
			}
			brInstr++
			if _, werr := stdin.Write(data); werr != nil {
				break
			}
		}
		done <- struct{}{}
	}()

	<-done
	log.Printf("H5 远程桌面(内置引擎)已断开：server=%d user=%d 转发=%d帧/%d字节 浏览器发送=%d条",
		info.ServerID, info.UserID, fwdFrames, fwdBytes, brInstr)
	// 给对端一点时间收尾，随后 defer 关闭 rdpd 进程与 WebSocket
	time.Sleep(100 * time.Millisecond)
}
