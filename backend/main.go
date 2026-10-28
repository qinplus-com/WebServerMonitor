package main

import (
	"context"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"qinjiagateway/brand"
	"qinjiagateway/config"
	"qinjiagateway/dao"
	"qinjiagateway/demo"
	"qinjiagateway/handler"
	"qinjiagateway/license"
	"qinjiagateway/middleware"
	"qinjiagateway/monitor"
	"qinjiagateway/notify"
	"qinjiagateway/utils"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

func main() {
	// 弹窗子进程：由 NotifyAsync 拉起，仅负责显示消息框后退出
	if notify.IsNotifyChild() {
		notify.ShowBlocking()
		return
	}

	var configFlag, dataFlag, aboutFlag, licInfoFlag string
	var setAdminUser, setAdminPass string
	var demoFlag bool
	flag.StringVar(&configFlag, "config", "", "配置文件路径（默认 exe 同目录 config.yaml）")
	flag.StringVar(&dataFlag, "data", "", "数据目录路径（默认 exe 同目录 data）")
	flag.StringVar(&aboutFlag, "about", "", "输出软件名称、服务名、公司名、网址后退出（管理工具调用）")
	flag.StringVar(&licInfoFlag, "licinfo", "", "输出授权状态key=value后退出（管理工具调用，可顺带激活license.lic）")
	// 安装程序专用：管理员凭据不写入 config.yaml（避免明文落盘），直接写入数据库用户表
	flag.StringVar(&setAdminUser, "set-admin-user", "", "管理员用户名（与 -set-admin-pass 同时提供时写入用户表后退出）")
	flag.StringVar(&setAdminPass, "set-admin-pass", "", "管理员密码（与 -set-admin-user 同时提供时写入用户表后退出）")
	flag.BoolVar(&demoFlag, "demo", false, "演示模式：内置模拟服务器/命令/监控，锁定管理员账号")
	flag.Parse()

	// 解析路径（服务模式 cwd 为 System32，必须按 exe 目录解析）
	configPath, dataDir := resolvePaths(configFlag, dataFlag)
	// 日志落盘须在加载配置前：配置加载失败也要留痕
	initFileLog(dataDir)
	if _, err := config.Load(configPath); err != nil {
		log.Fatalf("加载配置失败: %v", err)
	}
	// 注入数据目录：AES 密钥文件等随数据持久化的资源依赖此路径
	config.Global.DataDir = dataDir

	// 演示模式：flag 或 config.yaml 中 demo: true 均可启用
	if demoFlag || config.Global.Demo {
		demo.Enable()
		log.Println("演示模式已启用：内置模拟服务器，管理员账号锁定")
	}

	// -about：以 key=value（UTF-8）输出品牌信息后立即退出，供管理工具读取，
	// 避免在管理脚本中硬编码公司名称与网址
	if aboutFlag != "" {
		fmt.Printf("app=%s\n", config.Global.App.Name)
		fmt.Printf("service=%s\n", config.Global.App.ServiceName)
		fmt.Printf("company=%s\n", brand.Company())
		fmt.Printf("website=%s\n", brand.Website())
		return
	}

	normalizeDSN(dataDir)

	if err := os.MkdirAll(dataDir, 0755); err != nil {
		log.Fatalf("创建数据目录失败: %v", err)
	}

	// 初始化数据库
	if err := dao.InitDB(); err != nil {
		log.Fatalf("初始化数据库失败: %v", err)
	}
	// 主动初始化 AES 密钥（持久化到数据目录 .aeskey），须在加密迁移之前
	utils.InitCrypto()
	// 执行数据库迁移（表结构变更 + 敏感字段加密等）；失败则中止启动，避免半迁移状态
	if err := dao.RunMigrations(); err != nil {
		log.Fatalf("数据库迁移失败: %v", err)
	}
	// 安装程序 -set-admin 模式：把向导中设置的管理员账号直接写入用户表后退出
	// （凭据不落盘配置文件）；已有数据时更新用户表，而非重建库。
	if setAdminUser != "" && setAdminPass != "" {
		if err := dao.SetAdminAccount(setAdminUser, setAdminPass); err != nil {
			log.Fatalf("写入管理员账号失败: %v", err)
		}
		log.Printf("管理员账号已写入数据库")
		return
	}
	dao.InitAdmin()

	// 演示模式：注入模拟服务器、URL、demo 账号
	if demo.IsDemo() {
		demo.Bootstrap(dao.DB)
	}

	// 安装时置入的授权文件（数据目录 license.lic）自动激活。
	// -licinfo 模式由下方块显式激活并回传具体错误，跳过自动激活
	// （否则失败时文件被重命名为 .failed、错误被吞，管理工具无法获知原因）。
	// 演示模式：跳过授权文件激活
	if licInfoFlag == "" && !demo.IsDemo() {
		license.AutoActivateFromFile(dataDir)
	}
	// 初始化授权系统（必须在数据库初始化之后）
	if demo.IsDemo() {
		license.Init("demo")
	} else {
		license.Init(config.Global.License.Edition)
	}

	// -licinfo：以 key=value（UTF-8）输出当前授权状态后退出，供管理工具渲染。
	// 若数据目录存在 license.lic 则顺带激活——服务停止（如授权失败拒启）时也能完成授权配置。
	if licInfoFlag != "" {
		var actErr string
		licPath := filepath.Join(dataDir, "license.lic")
		if _, statErr := os.Stat(licPath); statErr == nil {
			if err := license.ActivateFromFile(licPath); err != nil {
				actErr = err.Error()
			}
			license.Init(config.Global.License.Edition)
		}
		if e := license.GetEdition(); e != nil {
			fmt.Printf("edition=%s\n", e.Edition)
			fmt.Printf("max_servers=%d\n", e.MaxServers)
			fmt.Printf("expire_at=%s\n", e.ExpireAt)
			fmt.Printf("issued_to=%s\n", e.IssuedTo)
			fmt.Printf("is_trial=%t\n", e.IsTrial)
			fmt.Printf("days_left=%d\n", e.DaysLeft)
		}
		var srvCount int64
		dao.DB.Table("servers").Count(&srvCount)
		fmt.Printf("servers=%d\n", srvCount)
		fmt.Printf("machine_hash=%s\n", license.GetMachineHash())
		fmt.Printf("activate_error=%s\n", actErr)
		return
	}

	// 个人版：启动时校验数据库中服务器数量，若超限（如直接改库绕过）则报错退出
	// 演示模式：跳过服务器数量限制
	if !demo.IsDemo() {
		license.CheckStartupLimit()
	}

	// 启动后台网址监控（每60秒一轮）
	monitor.StartBackground(dao.DB, time.Minute)

	gin.SetMode(config.Global.Server.Mode)
	r := buildRouter()
	addr := fmt.Sprintf(":%d", config.Global.Server.Port)

	// 服务模式：交给 SCM 托管（服务名读配置）
	if isWindowsServiceMode() {
		if err := runAsWindowsService(addr, r); err != nil {
			log.Fatalf("服务运行失败: %v", err)
		}
		return
	}

	// 控制台模式（解压版双击运行）
	srv := &http.Server{Addr: addr, Handler: r}
	go func() {
		log.Printf("%s 启动成功，监听端口: %d", config.Global.App.Name, config.Global.Server.Port)
		log.Printf("浏览器访问: http://localhost:%d/", config.Global.Server.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("服务启动失败: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("正在关闭服务...")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	srv.Shutdown(ctx)
}

func buildRouter() *gin.Engine {
	r := gin.Default()

	// CORS跨域
	r.Use(cors.New(cors.Config{
		AllowOrigins: []string{"*"},
		AllowMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders: []string{"Origin", "Content-Type", "Authorization"},
	}))

	api := r.Group("/api/v1")
	{
		// 公开接口
		api.GET("/system/info", handler.SystemInfo)
		api.POST("/login", handler.Login)
		api.POST("/logout", middleware.JWTAuth(), handler.Logout)

		// 需要认证
		auth := api.Group("")
		auth.Use(middleware.JWTAuth())
		{
			auth.GET("/user/info", handler.GetCurrentUser)
			auth.GET("/user/prefs", handler.UserPrefGet)
			auth.PUT("/user/prefs", handler.UserPrefSave)

			// 服务器管理
			auth.GET("/server/list", handler.ServerList)
			auth.GET("/server/group/list", handler.GroupList)
			auth.GET("/server/url/list", handler.UrlList)
			auth.GET("/server/grant/users", handler.ServerGrantUsers)

			// Web监控
			auth.GET("/monitor/status", handler.MonitorStatus)

			// 终端
			auth.POST("/terminal/connect", handler.TerminalConnect)
			auth.POST("/terminal/close", handler.TerminalClose)
			auth.GET("/terminal/history", handler.TerminalHistory)
			auth.GET("/session/online", handler.SessionOnline)

			// H5 远程桌面（Windows RDP）
			auth.GET("/rdp/token", handler.RDPToken)

			// SFTP
			auth.GET("/sftp/list", handler.SftpList)
			auth.POST("/sftp/operate", handler.SftpOperate)
			auth.POST("/sftp/upload", handler.SftpUpload)
			auth.GET("/sftp/download", handler.SftpDownload)

			// 日志
			auth.GET("/log/login", handler.LogLogin)
			auth.GET("/log/command", handler.LogCommand)
			auth.GET("/log/sftp", handler.LogSftp)
			auth.GET("/log/export", handler.LogExport)

			// 概览
			auth.GET("/dashboard", handler.Dashboard)

			// AI助手（免费供应商代理）
			auth.POST("/ai/chat", handler.AIChat)
		}

		// 管理员专属
		admin := api.Group("")
		admin.Use(middleware.JWTAuth(), middleware.AdminAuth())
		{
			admin.POST("/server/add", handler.ServerAdd)
			admin.PUT("/server/edit", handler.ServerEdit)
			admin.DELETE("/server/del", handler.ServerDel)
			admin.POST("/server/group/add", handler.GroupAdd)
			admin.DELETE("/server/group/del", handler.GroupDel)
			admin.POST("/server/url/save", handler.UrlSave)
			admin.DELETE("/server/url/del", handler.UrlDel)
			admin.POST("/server/grant/save", handler.ServerGrantSave)
			admin.POST("/monitor/check", handler.MonitorCheck)

			admin.GET("/user/list", handler.UserList)
			admin.POST("/user/add", handler.UserAdd)
			admin.PUT("/user/edit", handler.UserEdit)
			admin.DELETE("/user/del", handler.UserDel)
			admin.POST("/user/permission", handler.UserPermission)
			admin.GET("/user/servers", handler.UserServers)

			admin.POST("/session/kick", handler.SessionKick)

			// 告警设置
			admin.GET("/alert/setting", handler.AlertSettingGet)
			admin.PUT("/alert/setting", handler.AlertSettingSave)
			admin.POST("/alert/test", handler.AlertTest)

			// 授权管理
			admin.GET("/license/info", handler.LicenseInfo)
			admin.POST("/license/activate", handler.LicenseActivate)
		}
	}

	// WebSocket终端
	r.GET("/ws/terminal", handler.TerminalWebSocket)

	// WebSocket H5 远程桌面
	r.GET("/ws/rdp", handler.RDPSocket)

	// 静态资源（编译时内嵌的前端产物）
	staticSub, _ := fs.Sub(staticEmbed, "static")
	assetsFS, _ := fs.Sub(staticEmbed, "static/assets")
	featuresFS, _ := fs.Sub(staticEmbed, "static/features")
	r.StaticFS("/assets", http.FS(assetsFS))
	r.StaticFS("/features", http.FS(featuresFS))

	fileServer := http.FileServer(http.FS(staticSub))
	serveFile := func(name string) gin.HandlerFunc {
		return func(c *gin.Context) {
			c.Request.URL.Path = "/" + name
			fileServer.ServeHTTP(c.Writer, c.Request)
		}
	}
	r.GET("/logo.png", serveFile("logo.png"))
	r.GET("/bgm.mp4", serveFile("bgm.mp4"))
	r.GET("/favicon.ico", func(c *gin.Context) {
		c.Request.URL.Path = "/logo.png"
		fileServer.ServeHTTP(c.Writer, c.Request)
	})

	r.NoRoute(func(c *gin.Context) {
		// API/WS请求返回404，其他返回index.html支持前端路由
		if strings.HasPrefix(c.Request.URL.Path, "/api/") || strings.HasPrefix(c.Request.URL.Path, "/ws/") {
			c.JSON(404, gin.H{"code": 404, "msg": "接口不存在"})
			return
		}
		data, err := staticEmbed.ReadFile("static/index.html")
		if err != nil {
			c.String(404, "前端资源缺失")
			return
		}
		// index.html 不缓存：升级后浏览器必须重新拉取，否则旧页引用的
		// hash 资源可能已被清理导致白屏；hash 资源本身由文件服务缓存
		c.Header("Cache-Control", "no-cache")
		c.Data(200, "text/html; charset=utf-8", data)
	})

	return r
}
