package dao

import (
	"qinjiagateway/config"
	"qinjiagateway/model"
	"qinjiagateway/utils"
)

// InitAdmin 初始化默认管理员
func InitAdmin() {
	var count int64
	DB.Model(&model.User{}).Count(&count)
	if count > 0 {
		return
	}
	admin := &model.User{
		Username: config.Global.Admin.Username,
		Password: utils.HashPassword(config.Global.Admin.Password),
		Role:     "admin",
		Status:   1,
		Remark:   "系统默认管理员",
	}
	DB.Create(admin)
}

// SetAdminAccount 将安装向导中设置的管理员账号写入用户表
// （安装程序 -set-admin 模式调用，凭据不再写入 config.yaml 明文落盘）：
//   - 用户表为空（全新安装）→ 直接创建管理员；
//   - 已有数据（升级/重装）→ 更新用户表：同名用户仅重置密码；
//     否则更新最早创建的管理员账号为新用户名+密码；无管理员时补建。
func SetAdminAccount(username, password string) error {
	hash := utils.HashPassword(password)
	var count int64
	DB.Model(&model.User{}).Count(&count)
	if count == 0 {
		admin := &model.User{
			Username: username,
			Password: hash,
			Role:     "admin",
			Status:   1,
			Remark:   "安装程序创建",
		}
		return DB.Create(admin).Error
	}
	// 已有数据：更新用户表
	var u model.User
	// 目标用户名已存在：仅重置密码并确保启用
	if err := DB.Where("username = ?", username).First(&u).Error; err == nil {
		u.Password = hash
		u.Status = 1
		return DB.Save(&u).Error
	}
	// 更新最早创建的管理员账号（重命名 + 改密）
	if err := DB.Where("role = ?", "admin").Order("id").First(&u).Error; err != nil {
		// 无管理员账号（异常数据）：补建
		admin := &model.User{
			Username: username,
			Password: hash,
			Role:     "admin",
			Status:   1,
			Remark:   "安装程序创建",
		}
		return DB.Create(admin).Error
	}
	u.Username = username
	u.Password = hash
	u.Status = 1
	return DB.Save(&u).Error
}
