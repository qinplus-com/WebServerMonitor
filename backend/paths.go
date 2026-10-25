package main

import (
	"os"
	"path/filepath"

	"qinjiagateway/config"
)

// resolvePaths 解析配置文件与数据目录位置。
// 服务模式下工作目录是 C:\Windows\System32，因此统一按 exe 所在目录解析。
func resolvePaths(configFlag, dataFlag string) (string, string) {
	exePath, err := os.Executable()
	base := filepath.Dir(exePath)
	if err != nil {
		base, _ = os.Getwd()
	}

	// 配置文件：-config > QJ_CONFIG > exe同目录/config.yaml > 当前目录config.yaml
	configPath := configFlag
	if configPath == "" {
		configPath = os.Getenv("QJ_CONFIG")
	}
	if configPath == "" {
		cand := filepath.Join(base, "config.yaml")
		if _, err := os.Stat(cand); err == nil {
			configPath = cand
		}
	}
	if configPath == "" {
		configPath = "config.yaml"
	}

	// 数据目录：-data > QJ_DATA_DIR > exe同目录/data
	dataDir := dataFlag
	if dataDir == "" {
		dataDir = os.Getenv("QJ_DATA_DIR")
	}
	if dataDir == "" {
		dataDir = filepath.Join(base, "data")
	}
	return configPath, dataDir
}

// normalizeDSN 将相对的 SQLite 路径锚定到数据目录
func normalizeDSN(dataDir string) {
	dsn := config.Global.Database.DSN
	if dsn == "" {
		dsn = "gateway.db"
	}
	if !filepath.IsAbs(dsn) {
		dsn = filepath.Join(dataDir, filepath.Base(dsn))
	}
	config.Global.Database.DSN = dsn
}
