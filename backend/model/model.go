package model

import "time"

// User 用户表
type User struct {
	ID        uint       `gorm:"primaryKey" json:"id"`
	Username  string     `gorm:"uniqueIndex;size:64;not null" json:"username"`
	Password  string     `gorm:"size:255;not null" json:"-"`
	Role      string     `gorm:"size:32;default:user" json:"role"` // admin / operator / user
	Status    int        `gorm:"default:1" json:"status"`          // 1启用 0禁用
	Remark    string     `gorm:"size:255" json:"remark"`
	LastLogin *time.Time `json:"last_login"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
}

// ServerGroup 服务器分组
type ServerGroup struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Name      string    `gorm:"size:64;not null" json:"name"`
	Remark    string    `gorm:"size:255" json:"remark"`
	CreatedAt time.Time `json:"created_at"`
}

// Server 服务器资源
type Server struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	Name       string    `gorm:"size:128;not null" json:"name"`
	IP         string    `gorm:"size:64;not null" json:"ip"`
	Port       int       `gorm:"default:22" json:"port"`
	OsType     string    `gorm:"size:16;default:linux" json:"os_type"` // linux / windows
	VncPort    int       `gorm:"default:0" json:"vnc_port"`            // Linux VNC 端口（0=未开启远程桌面）
	SmbShare   string    `gorm:"size:64;default:''" json:"smb_share"`  // Windows 文件管理共享名（SMB 共享文件夹）
	SmbPort    int       `gorm:"default:0" json:"smb_port"`            // SMB 端口（0=默认 445，公网端口映射场景可自定义）
	Username   string    `gorm:"size:64;not null" json:"username"`
	AuthType   string    `gorm:"size:16;default:password" json:"auth_type"` // password / key
	Password   string    `gorm:"size:255" json:"password"`                  // 管理员可见，普通用户接口返回前清空
	PrivateKey string    `gorm:"type:text" json:"private_key,omitempty"`    // 管理员可见，普通用户接口返回前清空
	GroupID    uint      `gorm:"default:0" json:"group_id"`
	GroupName  string    `gorm:"-" json:"group_name"`
	Status     int       `gorm:"default:0" json:"status"` // 1在线 0离线
	Remark     string    `gorm:"size:255" json:"remark"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// UserPref 用户偏好设置（终端配色/字号/反空闲等，JSON字符串）
type UserPref struct {
	UserID uint   `gorm:"primaryKey" json:"user_id"`
	Theme  string `gorm:"type:text" json:"theme"`
}

// ServerUrl 服务器关联的网站地址（用于Web可用性监控）
type ServerUrl struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	ServerID  uint      `gorm:"index" json:"server_id"`
	Protocol  string    `gorm:"size:8;default:https" json:"protocol"` // http / https，默认https
	Domain    string    `gorm:"size:255;not null" json:"domain"`      // 域名或IP
	Port      int       `json:"port"`                                 // 0表示默认端口（https:443，http:80）
	CreatedAt time.Time `json:"created_at"`
}

// UserServer 用户服务器授权关联
type UserServer struct {
	ID       uint `gorm:"primaryKey" json:"id"`
	UserID   uint `gorm:"index" json:"user_id"`
	ServerID uint `gorm:"index" json:"server_id"`
}

// LoginLog 登录日志
type LoginLog struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Username  string    `gorm:"size:64" json:"username"`
	IP        string    `gorm:"size:64" json:"ip"`
	UserAgent string    `gorm:"size:512" json:"user_agent"`
	Status    int       `json:"status"` // 1成功 0失败
	Message   string    `gorm:"size:255" json:"message"`
	CreatedAt time.Time `json:"created_at"`
}

// CommandLog 终端命令日志
type CommandLog struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	UserID     uint      `gorm:"index" json:"user_id"`
	Username   string    `gorm:"size:64" json:"username"`
	ServerID   uint      `json:"server_id"`
	ServerName string    `gorm:"size:128" json:"server_name"`
	SessionID  string    `gorm:"size:64;index" json:"session_id"`
	Command    string    `gorm:"type:text" json:"command"`
	Result     string    `gorm:"type:text" json:"result"`
	CreatedAt  time.Time `json:"created_at"`
}

// SftpLog 文件操作日志
type SftpLog struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	UserID     uint      `json:"user_id"`
	Username   string    `gorm:"size:64" json:"username"`
	ServerID   uint      `json:"server_id"`
	ServerName string    `gorm:"size:128" json:"server_name"`
	Operate    string    `gorm:"size:32" json:"operate"` // upload/download/delete/mkdir/rename
	Path       string    `gorm:"size:1024" json:"path"`
	CreatedAt  time.Time `json:"created_at"`
}

// AlertSetting 告警通知设置（全局单条，channels 为 JSON 字符串，含各通道配置）
type AlertSetting struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Channels  string    `gorm:"type:text" json:"channels"` // JSON: 各通道 enabled 与参数
	UpdatedAt time.Time `json:"updated_at"`
}

// License 授权信息
type License struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	Edition    string    `gorm:"size:32" json:"edition"`    // personal | sme | enterprise
	MaxServers int       `json:"max_servers"`               // -1 不限
	ExpireAt   time.Time `json:"expire_at"`                 // 试用/授权到期时间
	IssuedTo   string    `gorm:"size:256" json:"issued_to"` // 被授权方名称
	Content    string    `gorm:"type:text" json:"content"`  // License JSON 原文（用于验签）
	Status     string    `gorm:"size:32" json:"status"`     // active | expired | revoked
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}
