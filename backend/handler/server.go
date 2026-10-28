package handler

import (
	"strings"

	"qinjiagateway/dao"
	"qinjiagateway/demo"
	"qinjiagateway/license"
	"qinjiagateway/model"
	"qinjiagateway/utils"

	"github.com/gin-gonic/gin"
)

// ServerList 获取授权服务器列表
func ServerList(c *gin.Context) {
	userId, _ := c.Get("user_id")
	role, _ := c.Get("role")
	groupId := c.Query("group_id")
	keyword := c.Query("keyword")

	var servers []model.Server
	query := dao.DB.Model(&model.Server{})

	if role != "admin" {
		// 普通用户只看授权的服务器
		var serverIds []uint
		dao.DB.Model(&model.UserServer{}).Where("user_id = ?", userId).Pluck("server_id", &serverIds)
		if len(serverIds) == 0 {
			utils.Success(c, []model.Server{})
			return
		}
		query = query.Where("id IN ?", serverIds)
	}
	if groupId != "" {
		query = query.Where("group_id = ?", groupId)
	}
	if keyword != "" {
		query = query.Where("name LIKE ? OR ip LIKE ? OR remark LIKE ?", "%"+keyword+"%", "%"+keyword+"%", "%"+keyword+"%")
	}
	query.Order("id DESC").Find(&servers)

	// 填充分组名称
	for i := range servers {
		var group model.ServerGroup
		if servers[i].GroupID > 0 {
			dao.DB.First(&group, servers[i].GroupID)
			servers[i].GroupName = group.Name
		}
		// 普通用户不下发密码/私钥；管理员解密后下发（供编辑回显）
		if role != "admin" {
			servers[i].Password = ""
			servers[i].PrivateKey = ""
		} else {
			// 解密失败（密钥已变更/数据损坏）：回显空而非密文，引导用户重新输入保存
			if plain, ok := utils.DecryptEx(servers[i].Password); ok {
				servers[i].Password = plain
			} else {
				servers[i].Password = ""
			}
			if plain, ok := utils.DecryptEx(servers[i].PrivateKey); ok {
				servers[i].PrivateKey = plain
			} else {
				servers[i].PrivateKey = ""
			}
		}
	}
	utils.Success(c, servers)
}

// ServerAdd 新增服务器
func ServerAdd(c *gin.Context) {
	// 演示模式：禁止修改服务器配置
	if demo.IsDemo() {
		utils.Fail(c, "演示模式：禁止修改服务器配置")
		return
	}
	// 授权检查：服务器数量限制
	if ok, msg := license.CanAddServer(); !ok {
		utils.Fail(c, msg)
		return
	}
	var req model.Server
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Fail(c, "参数错误")
		return
	}
	if req.AuthType == "key" && req.PrivateKey == "" {
		utils.Fail(c, "密钥认证必须填写私钥")
		return
	}
	// 服务器类型：linux / windows，默认 linux
	if req.OsType != "windows" {
		req.OsType = "linux"
	}
	// Windows 文件管理共享名（SMB）：去除首尾空白，Linux 忽略
	req.SmbShare = strings.TrimSpace(req.SmbShare)
	if req.OsType != "windows" {
		req.SmbShare = ""
	}
	if req.VncPort < 0 {
		req.VncPort = 0
	}
	if req.VncPort > 65535 {
		req.VncPort = 65535
	}
	if req.SmbPort < 0 {
		req.SmbPort = 0
	}
	if req.SmbPort > 65535 {
		req.SmbPort = 65535
	}
	if req.Port == 0 {
		if req.OsType == "windows" {
			req.Port = 3389
		} else {
			req.Port = 22
		}
	}
	// 入库前加密敏感字段
	req.Password = utils.Encrypt(req.Password)
	req.PrivateKey = utils.Encrypt(req.PrivateKey)
	if err := dao.DB.Create(&req).Error; err != nil {
		utils.Fail(c, "新增失败："+err.Error())
		return
	}
	utils.Success(c, req)
}

// ServerEdit 编辑服务器
func ServerEdit(c *gin.Context) {
	// 演示模式：禁止修改服务器配置
	if demo.IsDemo() {
		utils.Fail(c, "演示模式：禁止修改服务器配置")
		return
	}
	var req model.Server
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Fail(c, "参数错误")
		return
	}
	updates := map[string]interface{}{
		"name":     req.Name,
		"ip":       req.IP,
		"port":     req.Port,
		"username": req.Username,
		"auth_type": req.AuthType,
		"group_id": req.GroupID,
		"status":   req.Status,
		"remark":   req.Remark,
	}
	// 服务器类型（非空才更新，兼容旧版前端）
	if req.OsType != "" {
		updates["os_type"] = req.OsType
	}
	// Linux VNC 远程桌面端口（0=关闭）
	if req.VncPort < 0 {
		req.VncPort = 0
	}
	if req.VncPort > 65535 {
		req.VncPort = 65535
	}
	if req.SmbPort < 0 {
		req.SmbPort = 0
	}
	if req.SmbPort > 65535 {
		req.SmbPort = 65535
	}
	updates["vnc_port"] = req.VncPort
	updates["smb_port"] = req.SmbPort
	// Windows 文件管理共享名（SMB）：去除首尾空白；Linux/空值清空
	updates["smb_share"] = strings.TrimSpace(req.SmbShare)
	if req.OsType == "linux" {
		updates["smb_share"] = ""
	}
	// 空密码/空私钥表示不修改（前端掩码占位），保留原值；非空则加密更新
	if req.Password != "" {
		updates["password"] = utils.Encrypt(req.Password)
	}
	if req.PrivateKey != "" {
		updates["private_key"] = utils.Encrypt(req.PrivateKey)
	}
	if err := dao.DB.Model(&model.Server{}).Where("id = ?", req.ID).Updates(updates).Error; err != nil {
		utils.Fail(c, "编辑失败："+err.Error())
		return
	}
	utils.Success(c, nil)
}

// ServerDel 删除服务器
func ServerDel(c *gin.Context) {
	// 演示模式：禁止修改服务器配置
	if demo.IsDemo() {
		utils.Fail(c, "演示模式：禁止修改服务器配置")
		return
	}
	id := c.Query("id")
	if id == "" {
		utils.Fail(c, "ID不能为空")
		return
	}
	dao.DB.Where("server_id = ?", id).Delete(&model.UserServer{})
	dao.DB.Where("server_id = ?", id).Delete(&model.ServerUrl{})
	dao.DB.Delete(&model.Server{}, id)
	utils.Success(c, nil)
}

// GroupList 分组列表
func GroupList(c *gin.Context) {
	var groups []model.ServerGroup
	dao.DB.Order("id ASC").Find(&groups)
	utils.Success(c, groups)
}

// GroupAdd 新增分组
func GroupAdd(c *gin.Context) {
	var req model.ServerGroup
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Fail(c, "参数错误")
		return
	}
	dao.DB.Create(&req)
	utils.Success(c, req)
}

// GroupDel 删除分组
func GroupDel(c *gin.Context) {
	id := c.Query("id")
	dao.DB.Delete(&model.ServerGroup{}, id)
	dao.DB.Model(&model.Server{}).Where("group_id = ?", id).Update("group_id", 0)
	utils.Success(c, nil)
}
