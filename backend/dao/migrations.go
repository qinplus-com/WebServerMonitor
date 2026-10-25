package dao

import (
	"encoding/json"
	"log"
	"strings"

	"qinjiagateway/model"
	"qinjiagateway/utils"
)

// SchemaMigration 记录已执行的迁移版本
type SchemaMigration struct {
	Version   string `gorm:"primaryKey;size:64"`
	AppliedAt string `gorm:"size:32"`
}

// migrationFunc 迁移函数签名：返回 error 表示失败，将阻止启动
type migrationFunc func() error

// migrations 按版本号顺序排列；新增迁移只需在此追加
var migrations = []struct {
	version string
	run     migrationFunc
}{
	{"20261004_encrypt_server_secrets", migrateEncryptServerSecrets},
	{"20261006_encrypt_alert_secrets", migrateEncryptAlertSecrets},
}

// RunMigrations 执行所有未应用的迁移。任何迁移失败都返回 error，
// 由调用方决定是否中止启动（避免带着半迁移状态运行）。
func RunMigrations() error {
	if err := DB.AutoMigrate(&SchemaMigration{}); err != nil {
		return err
	}
	for _, m := range migrations {
		var cnt int64
		DB.Model(&SchemaMigration{}).Where("version = ?", m.version).Count(&cnt)
		if cnt > 0 {
			continue
		}
		log.Printf("[migrate] applying %s ...", m.version)
		if err := m.run(); err != nil {
			log.Printf("[migrate] %s FAILED: %v", m.version, err)
			return err
		}
		DB.Create(&SchemaMigration{Version: m.version, AppliedAt: "now"})
		log.Printf("[migrate] %s applied", m.version)
	}
	return nil
}

// migrateEncryptServerSecrets 将 servers 表中明文存储的 password/private_key
// 升级为 AES 密文。已是密文（带 ENC1: 前缀）的跳过，保证幂等。
func migrateEncryptServerSecrets() error {
	var servers []model.Server
	if err := DB.Find(&servers).Error; err != nil {
		return err
	}
	for _, s := range servers {
		changed := false
		if s.Password != "" && !isEncrypted(s.Password) {
			s.Password = utils.Encrypt(s.Password)
			changed = true
		}
		if s.PrivateKey != "" && !isEncrypted(s.PrivateKey) {
			s.PrivateKey = utils.Encrypt(s.PrivateKey)
			changed = true
		}
		if changed {
			if err := DB.Model(&model.Server{}).Where("id = ?", s.ID).
				Updates(map[string]interface{}{
					"password":    s.Password,
					"private_key": s.PrivateKey,
				}).Error; err != nil {
				return err
			}
		}
	}
	return nil
}

// isEncrypted 判断是否为已加密的密文
func isEncrypted(s string) bool {
	return len(s) > len("ENC1:") && s[:len("ENC1:")] == "ENC1:"
}

// alertSensitiveKeys 告警配置中需加密的敏感参数 key（与 notify.sensitiveParamKeys 保持一致）
var alertSensitiveKeys = map[string]bool{
	"pass":       true,
	"token":      true,
	"app_secret": true,
	"key":        true,
}

// migrateEncryptAlertSecrets 将 alert_settings.channels JSON 中的敏感字段（邮件授权码等）
// 由明文升级为 AES 密文。已是密文（带 ENC1: 前缀）的跳过，保证幂等。
func migrateEncryptAlertSecrets() error {
	var sets []model.AlertSetting
	if err := DB.Find(&sets).Error; err != nil {
		return err
	}
	for i := range sets {
		s := &sets[i]
		if strings.TrimSpace(s.Channels) == "" {
			continue
		}
		var raw map[string]json.RawMessage
		if err := json.Unmarshal([]byte(s.Channels), &raw); err != nil {
			continue
		}
		changed := false
		for chName, rawCh := range raw {
			var ch struct {
				Enabled bool                   `json:"enabled"`
				Params  map[string]interface{} `json:"params"`
			}
			if err := json.Unmarshal(rawCh, &ch); err != nil {
				continue
			}
			chChanged := false
			for k, v := range ch.Params {
				if !alertSensitiveKeys[k] {
					continue
				}
				sv, ok := v.(string)
				if !ok || sv == "" || isEncrypted(sv) {
					continue
				}
				ch.Params[k] = utils.Encrypt(sv)
				chChanged = true
			}
			if chChanged {
				newCh, err := json.Marshal(ch)
				if err == nil {
					raw[chName] = newCh
					changed = true
				}
			}
		}
		if changed {
			newData, err := json.Marshal(raw)
			if err == nil {
				if err := DB.Model(&model.AlertSetting{}).Where("id = ?", s.ID).
					Update("channels", string(newData)).Error; err != nil {
					return err
				}
			}
		}
	}
	return nil
}
