//go:build windows

package license

import (
	"fmt"
	"os/exec"
	"strings"
	"syscall"
)

// cimClass wmic component 名 → CIM 类名/属性名
var cimClass = map[string][2]string{
	"cpu":       {"Win32_Processor", "ProcessorId"},
	"baseboard": {"Win32_BaseBoard", "SerialNumber"},
}

// readCIMValue 通过 PowerShell CIM 读取硬件属性（wmic 被移除时的回退方案）
func readCIMValue(component string) string {
	m, ok := cimClass[component]
	if !ok {
		return ""
	}
	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command",
		fmt.Sprintf("(Get-CimInstance -ClassName %s | Select-Object -First 1 -ExpandProperty %s)", m[0], m[1]))
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: 0x08000000} // CREATE_NO_WINDOW
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}
