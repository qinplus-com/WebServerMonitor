package handler

import (
	"fmt"
	"io"
	"net"
	"os"
	"path"
	"strings"
	"time"

	"qinjiagateway/model"
	"qinjiagateway/utils"

	"github.com/gin-gonic/gin"
	"github.com/hirochachacha/go-smb2"
)

// Windows 服务器文件管理：通过 SMB 协议连接目标机的共享文件夹（445 端口，共享名
// 由用户在服务器编辑中配置，不使用 C$ 管理共享），与 Linux 的 SFTP 接口保持
// 相同的请求/响应结构，前端文件页无需区分平台。

// getSmbShare 建立 SMB 会话并挂载服务器配置的共享文件夹；返回共享句柄与清理函数。
// 不使用 C$/ADMIN$ 管理共享（受 UAC 远程令牌过滤限制），要求用户在目标机创建共享文件夹。
func getSmbShare(server *model.Server) (*smb2.Share, func(), error) {
	// 演示模式：模拟服务器不支持 SMB（无真实文件系统）
	if strings.HasPrefix(server.IP, "demo://") {
		return nil, nil, fmt.Errorf("演示模式：文件管理不可用（需要真实服务器）")
	}
	// 解密密码；失败说明加密密钥已变更，禁止拿密文尝试认证（必败且误导）
	if plain, ok := utils.DecryptEx(server.Password); ok {
		server.Password = plain
	} else {
		return nil, nil, fmt.Errorf("服务器密码无法解密（加密密钥已变更），请在服务器列表中重新编辑并保存该服务器密码")
	}
	// 账号支持 domain\user 写法（域环境）
	user, domain := server.Username, ""
	if i := strings.LastIndex(user, "\\"); i >= 0 {
		domain, user = user[:i], user[i+1:]
	}
	shareName := strings.TrimSpace(server.SmbShare)
	if shareName == "" {
		return nil, nil, fmt.Errorf("未配置文件共享：请先在目标机创建共享文件夹（右键文件夹 → 属性 → 共享 → 共享，账号授予读写权限），再到 服务器编辑 → 文件共享名 填写共享名称")
	}
	// SMB 端口：默认 445；公网端口映射场景（如公网 4450 → 内部 445）可自定义
	port := server.SmbPort
	if port <= 0 {
		port = 445
	}
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", server.IP, port), 10*time.Second)
	if err != nil {
		return nil, nil, fmt.Errorf("连接服务器 %d 端口失败: %v（目标机需开启 SMB 共享服务且防火墙/安全组已放行）", port, err)
	}
	d := &smb2.Dialer{
		Initiator: &smb2.NTLMInitiator{User: user, Password: server.Password, Domain: domain},
	}
	sess, err := d.Dial(conn)
	if err != nil {
		conn.Close()
		return nil, nil, fmt.Errorf("SMB 认证失败: %v（请确认服务器登录账号密码正确）", err)
	}
	share, err := sess.Mount(shareName)
	if err != nil {
		sess.Logoff()
		conn.Close()
		return nil, nil, fmt.Errorf("连接共享 %s 失败: %v。请在目标机检查：① 共享文件夹已创建（右键文件夹 → 属性 → 共享；或管理员 PowerShell 执行 New-SmbShare -Name %s -Path <目录> -FullAccess <账号>）；② 账号对该共享有读写权限；③ 防火墙已放行 SMB（445 端口）", shareName, err, shareName)
	}
	cleanup := func() {
		share.Umount()
		sess.Logoff()
		conn.Close()
	}
	return share, cleanup, nil
}

// smbPath 前端 POSIX 路径转 SMB 共享内相对路径："/a/b" → "a\b"，"/" → ""（共享根目录）
func smbPath(p string) string {
	p = strings.ReplaceAll(strings.TrimPrefix(path.Clean("/"+p), "/"), "/", "\\")
	if p == "." {
		return ""
	}
	return p
}

// smbJoin 拼接目录与名称为 POSIX 风格（与前端路径规则一致）
func smbJoin(dir, name string) string {
	if dir == "/" {
		return "/" + name
	}
	return dir + "/" + name
}

func smbList(c *gin.Context, server *model.Server, p string) {
	share, cleanup, err := getSmbShare(server)
	if err != nil {
		utils.Fail(c, err.Error())
		return
	}
	defer cleanup()

	entries, err := share.ReadDir(smbPath(p))
	if err != nil {
		utils.Fail(c, "读取目录失败: "+err.Error())
		return
	}
	type fileInfo struct {
		Name    string    `json:"name"`
		IsDir   bool      `json:"is_dir"`
		Size    int64     `json:"size"`
		ModTime time.Time `json:"mod_time"`
		Perm    string    `json:"perm"`
	}
	files := make([]fileInfo, 0, len(entries))
	for _, e := range entries {
		files = append(files, fileInfo{
			Name:    e.Name(),
			IsDir:   e.IsDir(),
			Size:    e.Size(),
			ModTime: e.ModTime(),
			Perm:    e.Mode().String(),
		})
	}
	utils.Success(c, gin.H{"path": p, "files": files})
}

func smbOperate(c *gin.Context, server *model.Server, p, operateType, newName string) {
	share, cleanup, err := getSmbShare(server)
	if err != nil {
		utils.Fail(c, err.Error())
		return
	}
	defer cleanup()

	smb := smbPath(p)
	switch operateType {
	case "mkdir":
		err = share.MkdirAll(smb, os.ModePerm)
	case "delete":
		// Remove 对文件与空目录均生效；非空目录由 SMB 返回错误
		err = share.Remove(smb)
	case "rename":
		err = share.Rename(smb, smbPath(smbJoin(path.Dir(p), newName)))
	default:
		utils.Fail(c, "不支持的操作类型")
		return
	}
	if err != nil {
		utils.Fail(c, "操作失败: "+err.Error())
		return
	}
	recordSftpLog(c, server, operateType, p)
	utils.Success(c, nil)
}

func smbUpload(c *gin.Context, server *model.Server, targetPath string, file io.Reader, headerSize int64, relativePath, filename string) {
	share, cleanup, err := getSmbShare(server)
	if err != nil {
		utils.Fail(c, err.Error())
		return
	}
	defer cleanup()

	// 解析目标路径：relative_path 非空时保留文件夹目录结构（文件夹上传）
	var dstPosix string
	if relativePath != "" {
		rel := path.Clean(strings.ReplaceAll(strings.TrimPrefix(relativePath, "/"), "\\", "/"))
		if rel == "." || rel == ".." || strings.HasPrefix(rel, "../") {
			utils.Fail(c, "相对路径非法")
			return
		}
		dstPosix = path.Join(targetPath, rel)
	} else {
		safeName := path.Base(filename)
		if safeName == "." || safeName == "/" || safeName == "\\" || safeName == "" {
			utils.Fail(c, "文件名非法")
			return
		}
		dstPosix = path.Join(targetPath, safeName)
	}
	// 文件夹上传场景：先递归创建父目录
	if dir := smbPath(path.Dir(dstPosix)); dir != "" {
		if err := share.MkdirAll(dir, os.ModePerm); err != nil {
			utils.Fail(c, "创建远程目录失败: "+err.Error())
			return
		}
	}
	dst, err := share.Create(smbPath(dstPosix))
	if err != nil {
		utils.Fail(c, "创建远程文件失败: "+err.Error())
		return
	}
	written, err := io.Copy(dst, file)
	if err != nil {
		dst.Close()
		utils.Fail(c, "上传失败: "+err.Error())
		return
	}
	dst.Close()
	if written != headerSize {
		utils.Fail(c, fmt.Sprintf("上传不完整: 已发送 %d 字节，应为 %d 字节", written, headerSize))
		return
	}
	if info, e := share.Stat(smbPath(dstPosix)); e == nil && info.Size() != headerSize {
		utils.Fail(c, fmt.Sprintf("服务器文件大小校验失败: 实际 %d 字节，应为 %d 字节", info.Size(), headerSize))
		return
	}
	recordSftpLog(c, server, "upload", dstPosix)
	utils.Success(c, gin.H{"path": dstPosix, "name": path.Base(dstPosix), "size": headerSize})
}

func smbDownload(c *gin.Context, server *model.Server, p string) {
	share, cleanup, err := getSmbShare(server)
	if err != nil {
		utils.Fail(c, err.Error())
		return
	}
	defer cleanup()

	src, err := share.Open(smbPath(p))
	if err != nil {
		utils.Fail(c, "打开远程文件失败: "+err.Error())
		return
	}
	defer src.Close()

	recordSftpLog(c, server, "download", p)
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", path.Base(p)))
	c.Header("Content-Type", "application/octet-stream")
	io.Copy(c.Writer, src)
}
