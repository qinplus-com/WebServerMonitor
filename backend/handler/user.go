package handler

import (
	"qinjiagateway/dao"
	"qinjiagateway/demo"
	"qinjiagateway/model"
	"qinjiagateway/utils"

	"github.com/gin-gonic/gin"
)

// UserPrefGet 获取当前用户偏好设置
func UserPrefGet(c *gin.Context) {
	userId, _ := c.Get("user_id")
	var pref model.UserPref
	if err := dao.DB.First(&pref, userId).Error; err != nil || pref.Theme == "" {
		utils.Success(c, nil)
		return
	}
	// 原样返回存储的JSON
	c.Data(200, "application/json", []byte(`{"code":200,"msg":"success","data":`+pref.Theme+`}`))
}

// UserPrefSave 保存当前用户偏好设置
func UserPrefSave(c *gin.Context) {
	userId, _ := c.Get("user_id")
	body, err := c.GetRawData()
	if err != nil || len(body) == 0 {
		utils.Fail(c, "参数错误")
		return
	}
	pref := model.UserPref{UserID: userId.(uint), Theme: string(body)}
	dao.DB.Save(&pref)
	utils.Success(c, nil)
}

// UserList 用户列表
func UserList(c *gin.Context) {
	keyword := c.Query("keyword")
	var users []model.User
	query := dao.DB.Model(&model.User{})
	if keyword != "" {
		query = query.Where("username LIKE ? OR remark LIKE ?", "%"+keyword+"%", "%"+keyword+"%")
	}
	query.Order("id DESC").Find(&users)

	// 查询每个用户已授权的服务器名称
	type grantRow struct {
		UserID     uint
		ServerName string
	}
	var grants []grantRow
	dao.DB.Table("user_servers").
		Select("user_servers.user_id as user_id, servers.name as server_name").
		Joins("LEFT JOIN servers ON servers.id = user_servers.server_id").
		Scan(&grants)
	userServers := make(map[uint][]string)
	for _, g := range grants {
		if g.ServerName != "" {
			userServers[g.UserID] = append(userServers[g.UserID], g.ServerName)
		}
	}

	// 过滤密码字段，返回时不带密码
	type safeUser struct {
		ID          uint     `json:"id"`
		Username    string   `json:"username"`
		Role        string   `json:"role"`
		Status      int      `json:"status"`
		Remark      string   `json:"remark"`
		ServerNames []string `json:"server_names"`
		CreatedAt   string   `json:"created_at"`
	}
	var list []safeUser
	for _, u := range users {
		list = append(list, safeUser{u.ID, u.Username, u.Role, u.Status, u.Remark, userServers[u.ID], u.CreatedAt.Format("2006-01-02 15:04:05")})
	}
	utils.Success(c, list)
}

// UserAdd 新增用户
func UserAdd(c *gin.Context) {
	var req struct {
		Username string `json:"username" binding:"required"`
		Password string `json:"password" binding:"required"`
		Role     string `json:"role"`
		Remark   string `json:"remark"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Fail(c, "参数错误")
		return
	}
	if req.Role == "" {
		req.Role = "user"
	}
	var cnt int64
	dao.DB.Model(&model.User{}).Where("username = ?", req.Username).Count(&cnt)
	if cnt > 0 {
		utils.Fail(c, "用户名已存在")
		return
	}
	user := model.User{
		Username: req.Username,
		Password: utils.HashPassword(req.Password),
		Role:     req.Role,
		Status:   1,
		Remark:   req.Remark,
	}
	dao.DB.Create(&user)
	utils.Success(c, gin.H{"id": user.ID, "username": user.Username})
}

// UserEdit 编辑用户
func UserEdit(c *gin.Context) {
	var req struct {
		ID       uint   `json:"id" binding:"required"`
		Password string `json:"password"`
		Role     string `json:"role"`
		Status   int    `json:"status"`
		Remark   string `json:"remark"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Fail(c, "参数错误")
		return
	}
	// 演示模式：禁止修改 demo 管理员账号
	if demo.IsDemo() {
		var u model.User
		if err := dao.DB.First(&u, req.ID).Error; err == nil && u.Username == "demo" {
			utils.Fail(c, "演示模式：禁止修改管理员账号")
			return
		}
	}
	updates := map[string]interface{}{
		"role":   req.Role,
		"status": req.Status,
		"remark": req.Remark,
	}
	if req.Password != "" {
		updates["password"] = utils.HashPassword(req.Password)
	}
	dao.DB.Model(&model.User{}).Where("id = ?", req.ID).Updates(updates)
	utils.Success(c, nil)
}

// UserDel 删除用户
func UserDel(c *gin.Context) {
	id := c.Query("id")
	// 演示模式：禁止删除 demo 管理员账号
	if demo.IsDemo() {
		var u model.User
		if err := dao.DB.First(&u, id).Error; err == nil && u.Username == "demo" {
			utils.Fail(c, "演示模式：禁止删除管理员账号")
			return
		}
	}
	dao.DB.Where("user_id = ?", id).Delete(&model.UserServer{})
	dao.DB.Delete(&model.User{}, id)
	utils.Success(c, nil)
}

// UserPermission 分配服务器权限
func UserPermission(c *gin.Context) {
	var req struct {
		UserID    uint   `json:"user_id" binding:"required"`
		ServerIDs []uint `json:"server_ids"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Fail(c, "参数错误")
		return
	}
	// 演示模式：禁止修改 demo 管理员权限
	if demo.IsDemo() {
		var u model.User
		if err := dao.DB.First(&u, req.UserID).Error; err == nil && u.Username == "demo" {
			utils.Fail(c, "演示模式：禁止修改管理员权限")
			return
		}
	}
	dao.DB.Where("user_id = ?", req.UserID).Delete(&model.UserServer{})
	for _, sid := range req.ServerIDs {
		dao.DB.Create(&model.UserServer{UserID: req.UserID, ServerID: sid})
	}
	utils.Success(c, nil)
}

// UserServers 获取用户已授权服务器ID
func UserServers(c *gin.Context) {
	userID := c.Query("user_id")
	var ids []uint
	dao.DB.Model(&model.UserServer{}).Where("user_id = ?", userID).Pluck("server_id", &ids)
	utils.Success(c, ids)
}
