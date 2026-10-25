package handler

import (
	"qinjiagateway/license"
	"qinjiagateway/utils"

	"github.com/gin-gonic/gin"
)

// LicenseInfo 获取当前授权版本信息
func LicenseInfo(c *gin.Context) {
	utils.Success(c, license.GetEdition())
}

// LicenseActivate 激活 License
func LicenseActivate(c *gin.Context) {
	var req struct {
		Content string `json:"content" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Fail(c, "请输入授权码")
		return
	}
	if err := license.Activate(req.Content); err != nil {
		utils.Fail(c, "激活失败: "+err.Error())
		return
	}
	utils.Success(c, license.GetEdition())
}
