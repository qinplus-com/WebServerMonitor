package handler

import (
	"qinjiagateway/dao"
	"qinjiagateway/model"
	"qinjiagateway/notify"
	"qinjiagateway/utils"

	"github.com/gin-gonic/gin"
)

// AlertSettingGet 获取告警通知配置
func AlertSettingGet(c *gin.Context) {
	cfg, err := notify.LoadConfig()
	if err != nil {
		utils.Fail(c, "读取配置失败: "+err.Error())
		return
	}
	// 敏感字段（邮件授权码等）以掩码返回，不泄露明文
	utils.Success(c, cfg.ForDisplay())
}

// AlertSettingSave 保存告警通知配置
func AlertSettingSave(c *gin.Context) {
	var cfg notify.Config
	if err := c.ShouldBindJSON(&cfg); err != nil {
		utils.Fail(c, "参数错误")
		return
	}
	// 读取已存配置：敏感字段留空或为掩码时，保留原值（前端「不修改留空」约定）
	oldCfg, _ := notify.LoadConfig()
	cfg.MaskForEmptyUpdate(oldCfg)
	data, err := cfg.EncryptForSave()
	if err != nil {
		utils.Fail(c, "配置序列化失败: "+err.Error())
		return
	}
	// 全局单条：存在则更新，不存在则创建
	var set model.AlertSetting
	result := dao.DB.Order("id ASC").First(&set)
	if result.Error != nil {
		set.Channels = string(data)
		if err := dao.DB.Create(&set).Error; err != nil {
			utils.Fail(c, "保存失败: "+err.Error())
			return
		}
	} else {
		if err := dao.DB.Model(&set).Update("channels", data).Error; err != nil {
			utils.Fail(c, "保存失败: "+err.Error())
			return
		}
	}
	utils.Success(c, nil)
}

// AlertTest 测试指定通道发送
func AlertTest(c *gin.Context) {
	var req struct {
		Channel string                 `json:"channel"`
		Params  map[string]interface{} `json:"params"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Fail(c, "参数错误")
		return
	}
	if err := notify.SendTest(req.Channel, req.Params); err != nil {
		utils.Fail(c, "发送失败: "+err.Error())
		return
	}
	utils.Success(c, nil)
}
