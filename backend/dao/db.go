package dao

import (
	"qinjiagateway/config"
	"qinjiagateway/model"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var DB *gorm.DB

func InitDB() error {
	db, err := gorm.Open(sqlite.Open(config.Global.Database.DSN), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
	if err != nil {
		return err
	}
	DB = db

	// 自动迁移表结构
	return db.AutoMigrate(
		&model.User{},
		&model.ServerGroup{},
		&model.Server{},
		&model.ServerUrl{},
		&model.UserServer{},
		&model.UserPref{},
		&model.LoginLog{},
		&model.CommandLog{},
		&model.SftpLog{},
		&model.AlertSetting{},
		&model.License{},
	)
}
