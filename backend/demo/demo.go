// Package demo 演示模式：内置模拟服务器、模拟命令、模拟监控数据，
// 用于向客户展示产品功能而无需真实服务器环境。
package demo

import (
	"log"
	"sync"

	"qinjiagateway/model"
	"qinjiagateway/utils"

	"gorm.io/gorm"
)

var (
	isDemo bool
	mu     sync.RWMutex
)

// Enable 启用演示模式（由 main.go 在加载配置后调用）
func Enable() {
	mu.Lock()
	isDemo = true
	mu.Unlock()
}

// IsDemo 查询当前是否为演示模式
func IsDemo() bool {
	mu.RLock()
	defer mu.RUnlock()
	return isDemo
}

// IsDemoServer 判断服务器是否为演示模式模拟服务器
func IsDemoServer(ip string) bool {
	return len(ip) >= 7 && ip[:7] == "demo://"
}

// IsDemoDomain 判断 URL 域名是否为演示模式模拟域名
func IsDemoDomain(domain string) bool {
	return len(domain) >= 11 && domain[len(domain)-11:] == ".demo.local"
}

// Bootstrap 注入演示数据（幂等，每次启动调用）
func Bootstrap(db *gorm.DB) {
	if !IsDemo() {
		return
	}
	log.Println("演示模式：开始注入演示数据...")

	// 1. 创建 demo 管理员账号（每次启动强制重置密码）
	var demoUser model.User
	result := db.Where("username = ?", "demo").First(&demoUser)
	if result.Error != nil {
		// 不存在则创建
		demoUser = model.User{
			Username: "demo",
			Password: utils.HashPassword("demo123"),
			Role:     "admin",
			Status:   1,
			Remark:   "演示账号（密码不可修改）",
		}
		if err := db.Create(&demoUser).Error; err != nil {
			log.Printf("演示模式：创建 demo 用户失败: %v", err)
			return
		}
		log.Printf("演示模式：创建 demo 用户 (ID=%d)", demoUser.ID)
	} else {
		// 存在则强制重置密码和状态
		demoUser.Password = utils.HashPassword("demo123")
		demoUser.Role = "admin"
		demoUser.Status = 1
		demoUser.Remark = "演示账号（密码不可修改）"
		if err := db.Save(&demoUser).Error; err != nil {
			log.Printf("演示模式：重置 demo 用户失败: %v", err)
		}
	}

	// 2. 注入演示服务器（6 台）
	demoServers := []model.Server{
		{Name: "演示-Web服务器-01", IP: "demo://web-01", Port: 22, Username: "root", AuthType: "password", Password: "demo", Status: 1, Remark: "Nginx + PHP 生产环境"},
		{Name: "演示-Web服务器-02", IP: "demo://web-02", Port: 22, Username: "root", AuthType: "password", Password: "demo", Status: 1, Remark: "Nginx + Node.js 生产环境"},
		{Name: "演示-数据库服务器", IP: "demo://db-01", Port: 22, Username: "root", AuthType: "password", Password: "demo", Status: 1, Remark: "MySQL 8.0 主库"},
		{Name: "演示-缓存服务器", IP: "demo://redis-01", Port: 22, Username: "root", AuthType: "password", Password: "demo", Status: 1, Remark: "Redis 7.0 缓存集群"},
		{Name: "演示-监控服务器", IP: "demo://monitor-01", Port: 22, Username: "root", AuthType: "password", Password: "demo", Status: 1, Remark: "Prometheus + Grafana"},
		{Name: "演示-备份服务器", IP: "demo://backup-01", Port: 22, Username: "root", AuthType: "password", Password: "demo", Status: 1, Remark: "每日全量备份"},
	}

	serverIDs := make([]uint, 0, len(demoServers))
	for _, srv := range demoServers {
		var existing model.Server
		err := db.Where("ip = ?", srv.IP).First(&existing).Error
		if err != nil {
			// 不存在则创建
			if err := db.Create(&srv).Error; err != nil {
				log.Printf("演示模式：创建服务器 %s 失败: %v", srv.Name, err)
				continue
			}
			serverIDs = append(serverIDs, srv.ID)
			log.Printf("演示模式：创建服务器 %s (ID=%d)", srv.Name, srv.ID)
		} else {
			// 存在则更新基本信息（保留用户可能修改的分组等）
			existing.Name = srv.Name
			existing.Status = 1
			existing.Remark = srv.Remark
			db.Save(&existing)
			serverIDs = append(serverIDs, existing.ID)
		}
	}

	// 3. 注入演示 URL（6 条，覆盖各种监控场景）
	demoURLs := []struct {
		ServerIdx int
		Protocol  string
		Domain    string
	}{
		{0, "https", "ok-180.demo.local"},      // 证书 180 天正常
		{0, "https", "warn-25.demo.local"},     // 证书 25 天临期
		{1, "https", "expired.demo.local"},     // 证书已过期
		{1, "http", "http200.demo.local"},      // 纯 HTTP 200
		{2, "https", "err503.demo.local"},      // HTTP 503 错误
		{3, "https", "timeout.demo.local"},     // 连接超时
	}

	for _, u := range demoURLs {
		if u.ServerIdx >= len(serverIDs) {
			continue
		}
		serverID := serverIDs[u.ServerIdx]
		var existing model.ServerUrl
		err := db.Where("domain = ?", u.Domain).First(&existing).Error
		if err != nil {
			// 不存在则创建
			url := model.ServerUrl{
				ServerID: serverID,
				Protocol: u.Protocol,
				Domain:   u.Domain,
				Port:     0,
			}
			if err := db.Create(&url).Error; err != nil {
				log.Printf("演示模式：创建 URL %s 失败: %v", u.Domain, err)
			} else {
				log.Printf("演示模式：创建 URL %s (ID=%d)", u.Domain, url.ID)
			}
		}
	}

	// 4. 给 demo 用户授权全部演示服务器
	for _, sid := range serverIDs {
		var count int64
		db.Model(&model.UserServer{}).Where("user_id = ? AND server_id = ?", demoUser.ID, sid).Count(&count)
		if count == 0 {
			db.Create(&model.UserServer{UserID: demoUser.ID, ServerID: sid})
		}
	}

	log.Printf("演示模式：数据注入完成，共 %d 台服务器、%d 条监控 URL", len(serverIDs), len(demoURLs))
}
