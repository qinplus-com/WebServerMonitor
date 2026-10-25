// 勤加软件 - 主机指纹采集工具（窗口版）
// 在客户主机上运行，自动采集 CPU ID / MAC / 主机名并计算机器指纹，
// 填写企业名称后导出指纹信息文件，发送给内蒙古勤加软件有限公司生成授权文件。
//
// 注意：机器指纹算法必须与 backend/license/license.go 的 GetMachineHash 完全一致，
// 否则生成的授权在目标主机上校验不通过。
package main

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"

	"github.com/lxn/walk"
	. "github.com/lxn/walk/declarative"
)

// FingerprintInfo 指纹信息文件内容（JSON，位于 FINGERPRINT_BEGIN/END 标记之间）
type FingerprintInfo struct {
	Product  string `json:"product"`
	Company  string `json:"company"`
	Hostname string `json:"hostname"`
	CPU      string `json:"cpu"`
	MAC      string `json:"mac"`
	Hash     string `json:"hash"`
	Time     string `json:"time"`
}

func main() {
	host, _ := os.Hostname()
	cpu := readWMICValue("cpu", "processorid")
	mac := firstMAC()
	hash := machineHash()

	var hostEdit, cpuEdit, macEdit, hashEdit, companyEdit *walk.LineEdit
	var statusLabel *walk.Label
	var mw *walk.MainWindow

	_, err := MainWindow{
		AssignTo: &mw,
		Title:    "勤加软件 - 主机指纹采集工具",
		Size:     Size{Width: 560, Height: 340},
		MinSize:  Size{Width: 560, Height: 340},
		Layout:   VBox{MarginsZero: false, Spacing: 8},
		Children: []Widget{
			GroupBox{
				Title:  "主机信息",
				Layout: Grid{Columns: 2, Spacing: 6},
				Children: []Widget{
					Label{Text: "主 机 名："}, LineEdit{AssignTo: &hostEdit, ReadOnly: true, Text: host},
					Label{Text: "CPU ID："}, LineEdit{AssignTo: &cpuEdit, ReadOnly: true, Text: cpu},
					Label{Text: "MAC 地址："}, LineEdit{AssignTo: &macEdit, ReadOnly: true, Text: mac},
					Label{Text: "机器指纹："}, LineEdit{AssignTo: &hashEdit, ReadOnly: true, Text: hash},
				},
			},
			GroupBox{
				Title:  "使用单位信息（必填，将写入授权书）",
				Layout: Grid{Columns: 2, Spacing: 6},
				Children: []Widget{
					Label{Text: "企业名称："}, LineEdit{AssignTo: &companyEdit, CueBanner: "请输入企业/单位中文全称"},
				},
			},
			Composite{
				Layout: HBox{MarginsZero: true},
				Children: []Widget{
					PushButton{Text: "导出指纹信息文件…", OnClicked: func() {
						exportFingerprint(mw, statusLabel, companyEdit.Text(), host, cpu, mac, hash)
					}},
				},
			},
			Label{Text: "导出后请将指纹信息文件发送给内蒙古勤加软件有限公司，以生成授权文件。"},
			Label{AssignTo: &statusLabel, Text: ""},
		},
	}.Run()
	if err != nil {
		walk.MsgBox(nil, "启动失败", err.Error(), walk.MsgBoxIconError)
	}
}

func exportFingerprint(owner walk.Form, status *walk.Label, company, host, cpu, mac, hash string) {
	company = strings.TrimSpace(company)
	if company == "" {
		walk.MsgBox(owner, "提示", "请先填写企业名称，再导出指纹信息文件。", walk.MsgBoxIconInformation)
		return
	}

	info := FingerprintInfo{
		Product:  "QinJiaGateway",
		Company:  company,
		Hostname: host,
		CPU:      cpu,
		MAC:      mac,
		Hash:     hash,
		Time:     time.Now().Format("2006-01-02 15:04:05"),
	}
	js, _ := json.Marshal(info)

	var sb strings.Builder
	sb.WriteString("# 勤加软件 - 主机指纹信息文件\r\n")
	sb.WriteString("# 产品：勤加软件WEB服务器运维管理平台（QinJiaGateway）\r\n")
	sb.WriteString("# 请将本文件完整发送给内蒙古勤加软件有限公司以生成授权文件\r\n")
	sb.WriteString("# 本文件为 UTF-8 编码，请勿手动修改\r\n\r\n")
	sb.WriteString("FINGERPRINT_BEGIN\r\n")
	sb.Write(js)
	sb.WriteString("\r\nFINGERPRINT_END\r\n")

	dlg := new(walk.FileDialog)
	dlg.Title = "导出指纹信息文件"
	dlg.Filter = "指纹信息文件 (*.txt)|*.txt"
	dlg.FilePath = fmt.Sprintf("指纹信息-%s.txt", host)
	if ok, err := dlg.ShowSave(owner); err != nil || !ok {
		return
	}
	// UTF-8 带 BOM，保证记事本正确显示中文
	data := append([]byte{0xEF, 0xBB, 0xBF}, []byte(sb.String())...)
	if err := os.WriteFile(dlg.FilePath, data, 0644); err != nil {
		walk.MsgBox(owner, "导出失败", err.Error(), walk.MsgBoxIconError)
		return
	}
	status.SetText("已导出: " + dlg.FilePath)
	walk.MsgBox(owner, "导出成功", "指纹信息文件已保存：\n"+dlg.FilePath+"\n\n请将该文件发送给内蒙古勤加软件有限公司。", walk.MsgBoxIconInformation)
}

// machineHash 机器指纹：CPU ID + 第一块非回环网卡 MAC，sha256 取前 8 字节
// 与 backend/license/license.go GetMachineHash 保持一致
func machineHash() string {
	parts := collectParts()
	h := sha256.Sum256([]byte(strings.Join(parts, "|")))
	return fmt.Sprintf("%x", h[:8])
}

func collectParts() []string {
	var parts []string
	if v := readWMICValue("cpu", "processorid"); v != "" {
		parts = append(parts, "CPU:"+v)
	}
	if mac := firstMAC(); mac != "" {
		parts = append(parts, "MAC:"+mac)
	}
	if len(parts) == 0 {
		if v := readWMICValue("baseboard", "serialnumber"); v != "" {
			parts = append(parts, "BOARD:"+v)
		}
	}
	if len(parts) == 0 {
		parts = append(parts, "HOST:"+os.Getenv("COMPUTERNAME"))
	}
	return parts
}

func firstMAC() string {
	if ifaces, err := net.Interfaces(); err == nil {
		for _, iface := range ifaces {
			if iface.Flags&net.FlagLoopback != 0 || len(iface.HardwareAddr) == 0 {
				continue
			}
			return iface.HardwareAddr.String()
		}
	}
	return ""
}

// readWMICValue 读取硬件属性：优先 wmic，wmic 被移除/取不到时回退 PowerShell CIM
// （Windows 11 24H2 起 wmic 默认不再安装）
func readWMICValue(component, property string) string {
	if v, ok := execWMIC(component, property); ok && v != "" {
		return v
	}
	return readCIMValue(component)
}

func execWMIC(component, property string) (string, bool) {
	cmd := exec.Command("wmic", component, "get", property, "/value")
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: 0x08000000} // CREATE_NO_WINDOW
	out, err := cmd.Output()
	if err != nil {
		return "", false
	}
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, property+"=") {
			return strings.TrimSpace(strings.TrimPrefix(line, property+"=")), true
		}
	}
	return "", true
}

// cimClass wmic component 名 → CIM 类名/属性名
var cimClass = map[string][2]string{
	"cpu":       {"Win32_Processor", "ProcessorId"},
	"baseboard": {"Win32_BaseBoard", "SerialNumber"},
}

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
