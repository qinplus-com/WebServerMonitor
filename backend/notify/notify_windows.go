//go:build windows

// Package notify 提供服务进程向交互桌面弹出提示框的能力。
//
// Windows 服务运行在 Session 0，普通 MessageBox 对用户不可见；
// 因此采用「独立子进程承载弹窗」的方式：
//   - 父进程（服务）通过 NotifyAsync 拉起自身的一个子进程并传环境变量，不阻塞、不等待；
//   - 子进程 main 中检测到环境变量后调用 ShowBlocking 弹出 MessageBox，
//     服务随后可正常退出/继续运行，弹窗独立存留直至用户点击确定。
//   - 子进程位于 Session 0 时使用 MB_SERVICE_NOTIFICATION 将弹窗投递到交互桌面。
package notify

import (
	"log"
	"os"
	"os/exec"
	"syscall"
	"unsafe"
)

const (
	envTitle = "QJ_MSGBOX_TITLE"
	envText  = "QJ_MSGBOX_TEXT"
	envIcon  = "QJ_MSGBOX_ICON" // "error" | "warn"

	mbOK                  = 0x00000000
	mbIconError           = 0x00000010
	mbIconWarning         = 0x00000030
	mbTopmost             = 0x00040000
	mbSetForeground       = 0x00010000
	mbServiceNotification = 0x00200000
)

// NotifyAsync 以独立子进程弹出消息框（非阻塞），服务退出后弹窗仍保留
func NotifyAsync(title, text string, iconError bool) {
	exe, err := os.Executable()
	if err != nil {
		return
	}
	icon := "warn"
	if iconError {
		icon = "error"
	}
	cmd := exec.Command(exe)
	cmd.Env = append(os.Environ(),
		envTitle+"="+title,
		envText+"="+text,
		envIcon+"="+icon,
	)
	// CREATE_NO_WINDOW：避免子进程闪现控制台窗口
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
	if err := cmd.Start(); err != nil {
		log.Printf("[notify] 弹窗子进程启动失败: %v", err)
	}
}

// IsNotifyChild 判断当前进程是否为弹窗子进程
func IsNotifyChild() bool {
	return os.Getenv(envText) != ""
}

// ShowBlocking 弹出模态消息框，用户点击确定后返回
func ShowBlocking() {
	title := os.Getenv(envTitle)
	if title == "" {
		title = "提示"
	}
	text := os.Getenv(envText)
	flags := uintptr(mbOK | mbTopmost | mbSetForeground)
	if os.Getenv(envIcon) == "error" {
		flags |= uintptr(mbIconError)
	} else {
		flags |= uintptr(mbIconWarning)
	}
	if isSessionZero() {
		// Session 0（服务）：必须用服务通知标志投递到交互桌面
		flags |= uintptr(mbServiceNotification)
	}
	titlePtr, err := syscall.UTF16PtrFromString(title)
	if err != nil {
		return
	}
	textPtr, err := syscall.UTF16PtrFromString(text)
	if err != nil {
		return
	}
	user32 := syscall.NewLazyDLL("user32.dll")
	MessageBoxW := user32.NewProc("MessageBoxW")
	_, _, _ = MessageBoxW.Call(
		0,
		uintptr(unsafe.Pointer(titlePtr)),
		uintptr(unsafe.Pointer(textPtr)),
		flags,
	)
}

// isSessionZero 判断当前进程是否运行在 Session 0（服务会话）
func isSessionZero() bool {
	kernel32 := syscall.NewLazyDLL("kernel32.dll")
	pidToSession := kernel32.NewProc("ProcessIdToSessionId")
	var sid uint32
	r, _, _ := pidToSession.Call(uintptr(os.Getpid()), uintptr(unsafe.Pointer(&sid)))
	return r != 0 && sid == 0
}
