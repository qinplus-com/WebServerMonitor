package handler

import (
	"time"

	"qinjiagateway/dao"
	"qinjiagateway/model"
	"qinjiagateway/utils"

	"github.com/gin-gonic/gin"
)

// Login 用户登录
func Login(c *gin.Context) {
	var req struct {
		Username string `json:"username" binding:"required"`
		Password string `json:"password" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Fail(c, "参数错误")
		return
	}
	var user model.User
	if err := dao.DB.Where("username = ?", req.Username).First(&user).Error; err != nil {
		dao.DB.Create(&model.LoginLog{
			Username:  req.Username,
			IP:        c.ClientIP(),
			UserAgent: c.GetHeader("User-Agent"),
			Status:    0,
			Message:   "用户不存在",
		})
		utils.Fail(c, "用户名或密码错误")
		return
	}
	if user.Status != 1 {
		utils.Fail(c, "账号已被禁用")
		return
	}
	if !utils.CheckPassword(req.Password, user.Password) {
		dao.DB.Create(&model.LoginLog{
			Username:  req.Username,
			IP:        c.ClientIP(),
			UserAgent: c.GetHeader("User-Agent"),
			Status:    0,
			Message:   "密码错误",
		})
		utils.Fail(c, "用户名或密码错误")
		return
	}
	token, expire, err := utils.GenerateToken(&user)
	if err != nil {
		utils.Fail(c, "Token生成失败")
		return
	}
	now := time.Now()
	dao.DB.Model(&user).Update("last_login", &now)
	dao.DB.Create(&model.LoginLog{
		Username:  user.Username,
		IP:        c.ClientIP(),
		UserAgent: c.GetHeader("User-Agent"),
		Status:    1,
		Message:   "登录成功",
	})
	utils.Success(c, gin.H{
		"token":    token,
		"role":     user.Role,
		"username": user.Username,
		"expire":   expire,
	})
}

// Logout 退出登录
func Logout(c *gin.Context) {
	utils.Success(c, nil)
}

// GetCurrentUser 获取当前用户信息
func GetCurrentUser(c *gin.Context) {
	userId, _ := c.Get("user_id")
	var user model.User
	dao.DB.First(&user, userId)
	utils.Success(c, gin.H{
		"id":       user.ID,
		"username": user.Username,
		"role":     user.Role,
		"remark":   user.Remark,
	})
}
