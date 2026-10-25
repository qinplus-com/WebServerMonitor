package handler

import (
	"fmt"
	"strings"

	"qinjiagateway/dao"
	"qinjiagateway/demo"
	"qinjiagateway/model"
	"qinjiagateway/monitor"
	"qinjiagateway/utils"

	"github.com/gin-gonic/gin"
)

// UrlList 获取某服务器的网址列表
func UrlList(c *gin.Context) {
	serverID := c.Query("server_id")
	if serverID == "" {
		utils.Fail(c, "server_id不能为空")
		return
	}
	var urls []model.ServerUrl
	dao.DB.Where("server_id = ?", serverID).Order("id ASC").Find(&urls)
	utils.Success(c, urls)
}

// UrlSave 保存服务器的网址列表（整体替换）
func UrlSave(c *gin.Context) {
	// 演示模式：禁止修改监控配置
	if demo.IsDemo() {
		utils.Fail(c, "演示模式：禁止修改监控配置")
		return
	}
	var req struct {
		ServerID uint              `json:"server_id" binding:"required"`
		Urls     []model.ServerUrl `json:"urls"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Fail(c, "参数错误")
		return
	}
	// 校验服务器存在
	var cnt int64
	dao.DB.Model(&model.Server{}).Where("id = ?", req.ServerID).Count(&cnt)
	if cnt == 0 {
		utils.Fail(c, "服务器不存在")
		return
	}

	// 整体替换
	dao.DB.Where("server_id = ?", req.ServerID).Delete(&model.ServerUrl{})
	saved := make([]model.ServerUrl, 0, len(req.Urls))
	for _, u := range req.Urls {
		domain := strings.TrimSpace(u.Domain)
		if domain == "" {
			continue
		}
		// 清洗：去掉协议前缀和路径
		domain = strings.TrimPrefix(domain, "https://")
		domain = strings.TrimPrefix(domain, "http://")
		if i := strings.Index(domain, "/"); i >= 0 {
			domain = domain[:i]
		}
		// 域名中带冒号端口时拆分
		if i := strings.LastIndex(domain, ":"); i > 0 {
			var p int
			if _, err := fmt.Sscanf(domain[i+1:], "%d", &p); err == nil && p > 0 && p <= 65535 {
				if u.Port == 0 {
					u.Port = p
				}
				domain = domain[:i]
			}
		}
		if u.Protocol != "http" && u.Protocol != "https" {
			u.Protocol = "https"
		}
		row := model.ServerUrl{
			ServerID: req.ServerID,
			Protocol: u.Protocol,
			Domain:   domain,
			Port:     u.Port,
		}
		dao.DB.Create(&row)
		saved = append(saved, row)
	}

	// 触发一轮异步检测
	monitor.CheckAll(dao.DB, true)
	utils.Success(c, saved)
}

// UrlDel 删除单个网址
func UrlDel(c *gin.Context) {
	// 演示模式：禁止修改监控配置
	if demo.IsDemo() {
		utils.Fail(c, "演示模式：禁止修改监控配置")
		return
	}
	id := c.Query("id")
	dao.DB.Delete(&model.ServerUrl{}, id)
	utils.Success(c, nil)
}

// MonitorStatus 获取所有网址的最新监控状态（概览页）
func MonitorStatus(c *gin.Context) {
	userId, _ := c.Get("user_id")
	role, _ := c.Get("role")

	query := dao.DB.Model(&model.ServerUrl{})
	if role != "admin" {
		var serverIds []uint
		dao.DB.Model(&model.UserServer{}).Where("user_id = ?", userId).Pluck("server_id", &serverIds)
		if len(serverIds) == 0 {
			utils.Success(c, []interface{}{})
			return
		}
		query = query.Where("server_id IN ?", serverIds)
	}
	var urls []model.ServerUrl
	query.Order("server_id ASC, id ASC").Find(&urls)

	resultMap := monitor.GetResults()
	type item struct {
		model.ServerUrl
		ServerName string             `json:"server_name"`
		ServerIP   string             `json:"server_ip"`
		Status     *monitor.UrlStatus `json:"status"`
	}
	list := make([]item, 0, len(urls))
	for _, u := range urls {
		var server model.Server
		dao.DB.First(&server, u.ServerID)
		it := item{ServerUrl: u, ServerName: server.Name, ServerIP: server.IP, Status: resultMap[u.ID]}
		list = append(list, it)
	}
	utils.Success(c, list)
}

// MonitorCheck 手动立即检测全部网址（同步等待结果）
func MonitorCheck(c *gin.Context) {
	monitor.CheckAll(dao.DB, false)
	MonitorStatus(c)
}

// ServerGrantUsers 获取某服务器已授权的用户ID列表
func ServerGrantUsers(c *gin.Context) {
	serverID := c.Query("server_id")
	var ids []uint
	dao.DB.Model(&model.UserServer{}).Where("server_id = ?", serverID).Pluck("user_id", &ids)
	utils.Success(c, ids)
}

// ServerGrantSave 保存某服务器的授权用户列表（整体替换）
func ServerGrantSave(c *gin.Context) {
	var req struct {
		ServerID uint   `json:"server_id" binding:"required"`
		UserIDs  []uint `json:"user_ids"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Fail(c, "参数错误")
		return
	}
	dao.DB.Where("server_id = ?", req.ServerID).Delete(&model.UserServer{})
	for _, uid := range req.UserIDs {
		dao.DB.Create(&model.UserServer{UserID: uid, ServerID: req.ServerID})
	}
	utils.Success(c, nil)
}
