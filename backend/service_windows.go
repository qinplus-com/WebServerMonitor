//go:build windows

package main

import (
	"context"
	"log"
	"net/http"
	"time"

	"qinjiagateway/config"

	"github.com/gin-gonic/gin"
	"golang.org/x/sys/windows/svc"
)

// isWindowsServiceMode 判断是否由 Windows 服务控制管理器(SCM)启动
func isWindowsServiceMode() bool {
	ok, err := svc.IsWindowsService()
	return err == nil && ok
}

// runAsWindowsService 以 Windows 服务方式运行，阻塞至服务停止；
// 服务名读取自配置文件 app.service_name（安装时可自定义）
func runAsWindowsService(addr string, router *gin.Engine) error {
	return svc.Run(config.Global.App.ServiceName, &qjService{addr: addr, router: router})
}

type qjService struct {
	addr   string
	router *gin.Engine
}

// Execute 实现 svc.Handler：响应 SCM 的启动/停止/关机指令
func (s *qjService) Execute(args []string, r <-chan svc.ChangeRequest, changes chan<- svc.Status) (bool, uint32) {
	changes <- svc.Status{State: svc.StartPending}

	srv := &http.Server{Addr: s.addr, Handler: s.router}
	go func() {
		log.Printf("%s 服务启动，监听 %s", config.Global.App.Name, s.addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("HTTP服务异常: %v", err)
		}
	}()

	changes <- svc.Status{State: svc.Running, Accepts: svc.AcceptStop | svc.AcceptShutdown}

loop:
	for {
		select {
		case req := <-r:
			switch req.Cmd {
			case svc.Interrogate:
				changes <- req.CurrentStatus
			case svc.Stop, svc.Shutdown:
				break loop
			}
		}
	}

	changes <- svc.Status{State: svc.StopPending}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	srv.Shutdown(ctx)
	log.Println("服务已停止")
	return false, 0
}
