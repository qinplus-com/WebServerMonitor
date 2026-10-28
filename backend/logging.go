package main

import (
	"io"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// dateFileWriter 按天分割的日志文件写入器（数据目录 logs\gateway-YYYYMMDD.log）
type dateFileWriter struct {
	mu  sync.Mutex
	dir string
	day string
	f   *os.File
}

func (w *dateFileWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	today := time.Now().Format("20060102")
	if w.f == nil || w.day != today {
		if w.f != nil {
			w.f.Close()
		}
		f, err := os.OpenFile(filepath.Join(w.dir, "gateway-"+today+".log"),
			os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
		if err != nil {
			return 0, err
		}
		w.f, w.day = f, today
	}
	return w.f.Write(p)
}

// stderrSafe 包装 os.Stderr：服务模式下 stderr 是无效句柄，写入会报错；
// io.MultiWriter 遇到错误会短路导致后续文件不落盘，这里吞掉 stderr 的写错误
// （控制台模式下仍正常输出，服务模式下静默丢弃）。
type stderrSafe struct{}

func (stderrSafe) Write(p []byte) (int, error) {
	n, _ := os.Stderr.Write(p)
	return n, nil
}

// initFileLog 将进程日志落盘到 <数据目录>\logs\（按天分割）。
// 服务模式下 stderr 不可见，落盘是排障（如授权校验失败、启动超限）的唯一途径；
// 文件打不开时静默回退 stderr，不阻塞启动。同时接管 GIN 请求日志。
func initFileLog(dataDir string) {
	dir := filepath.Join(dataDir, "logs")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return
	}
	w := &dateFileWriter{dir: dir}
	mw := io.MultiWriter(w, stderrSafe{})
	log.SetOutput(mw)
	gin.DefaultWriter = mw
	gin.DefaultErrorWriter = mw
}
