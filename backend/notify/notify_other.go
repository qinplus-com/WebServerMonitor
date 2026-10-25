//go:build !windows

// Package notify 非 Windows 平台无弹窗实现（no-op）
package notify

// NotifyAsync 非 Windows 平台 no-op
func NotifyAsync(title, text string, iconError bool) {}

// IsNotifyChild 非 Windows 平台恒为 false
func IsNotifyChild() bool { return false }

// ShowBlocking 非 Windows 平台 no-op
func ShowBlocking() {}
