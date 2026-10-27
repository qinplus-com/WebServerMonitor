package config

import (
	"os"
	"strconv"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server   ServerConfig   `yaml:"server"`
	Database DatabaseConfig `yaml:"database"`
	JWT      JWTConfig      `yaml:"jwt"`
	Admin    AdminConfig    `yaml:"admin"`
	App      AppConfig      `yaml:"app"`
	License  LicenseConfig  `yaml:"license"`
	Demo     bool           `yaml:"demo"` // 演示模式：内置模拟服务器/命令/监控，锁定管理员账号
	// DataDir 数据目录（命令行 -data / QJ_DATA_DIR 注入，不入配置文件）；
	// 用于 AES 密钥文件等需随数据持久化、与安装目录解耦的资源
	DataDir string `yaml:"-"`
}

type LicenseConfig struct {
	Edition string `yaml:"edition"` // personal | sme | enterprise
}

type ServerConfig struct {
	Port int    `yaml:"port"`
	Mode string `yaml:"mode"`
	// GuacdAddr guacamole-server（guacd）地址，用于 H5 远程桌面（RDP）。
	// 默认 127.0.0.1:4822；置为 "-" 表示禁用（H5 入口提示不可用）
	GuacdAddr string `yaml:"guacd_addr"`
}

type DatabaseConfig struct {
	Driver string `yaml:"driver"`
	DSN    string `yaml:"dsn"`
}

type JWTConfig struct {
	Secret        string `yaml:"secret"`
	Expire        int64  `yaml:"expire"`
	RefreshExpire int64  `yaml:"refresh_expire"`
}

type AdminConfig struct {
	Username string `yaml:"username"`
	Password string `yaml:"password"`
}

// AppConfig 软件品牌信息（安装时可自定义）
type AppConfig struct {
	Name        string `yaml:"name"`         // 软件显示名称（平台标题、快捷方式名）
	ServiceName string `yaml:"service_name"` // Windows 系统服务名（仅字母数字_-）
}

var Global *Config

// DefaultConfig 内置默认配置（配置文件缺失时使用）
func DefaultConfig() *Config {
	return &Config{
		Server:   ServerConfig{Port: 8080, Mode: "release", GuacdAddr: "127.0.0.1:4822"},
		Database: DatabaseConfig{Driver: "sqlite", DSN: "gateway.db"},
		JWT:      JWTConfig{Secret: "qinjiagateway-default-secret", Expire: 7200, RefreshExpire: 604800},
		Admin:    AdminConfig{Username: "admin", Password: "admin123"},
		App:      AppConfig{Name: "勤加软件WEB服务器运维管理平台", ServiceName: "QinJiaGateway"},
		License:  LicenseConfig{Edition: "personal"},
	}
}

func Load(path string) (*Config, error) {
	cfg := DefaultConfig()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			// 配置文件不存在：使用内置默认值
			Global = cfg
			applyEnv(cfg)
			return cfg, nil
		}
		return nil, err
	}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, err
	}
	Global = cfg
	applyEnv(cfg)
	return cfg, nil
}

// applyEnv 环境变量覆盖（安装版/容器场景使用）：
// QJ_PORT / QJ_ADMIN_USER / QJ_ADMIN_PASS / QJ_JWT_SECRET
func applyEnv(cfg *Config) {
	if v := os.Getenv("QJ_PORT"); v != "" {
		if p, err := strconv.Atoi(v); err == nil && p > 0 {
			cfg.Server.Port = p
		}
	}
	if v := os.Getenv("QJ_ADMIN_USER"); v != "" {
		cfg.Admin.Username = v
	}
	if v := os.Getenv("QJ_ADMIN_PASS"); v != "" {
		cfg.Admin.Password = v
	}
	if v := os.Getenv("QJ_JWT_SECRET"); v != "" {
		cfg.JWT.Secret = v
	}
}
