//go:build !windows

package license

// readCIMValue 非 Windows 平台无硬件 CIM 数据源
func readCIMValue(component string) string {
	return ""
}
