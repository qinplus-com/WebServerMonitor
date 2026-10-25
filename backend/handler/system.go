package handler

import (
	"qinjiagateway/brand"
	"qinjiagateway/config"

	"github.com/gin-gonic/gin"
)

// SystemInfo 公开接口（无需登录）：返回软件名称与品牌信息
// 登录页、平台标题、关于页及管理工具均从此接口获取名称
func SystemInfo(c *gin.Context) {
	c.JSON(200, gin.H{
		"code": 200,
		"data": gin.H{
			"app_name":  config.Global.App.Name,
			"company":   brand.Company(),
			"website":   brand.Website(),
			"copyright": brand.Copyright(),
		},
	})
}
