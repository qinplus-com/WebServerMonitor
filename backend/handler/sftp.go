package handler

import (
	"fmt"
	"io"
	"os"
	"path"
	"strings"
	"time"

	"qinjiagateway/dao"
	"qinjiagateway/model"
	sshmgr "qinjiagateway/ssh"
	"qinjiagateway/utils"

	"github.com/gin-gonic/gin"
	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

// getSftpClient 为指定服务器创建SFTP客户端
func getSftpClient(server *model.Server) (*sftp.Client, *ssh.Client, error) {
	// 演示模式：模拟服务器不支持 SFTP（无真实文件系统）
	if strings.HasPrefix(server.IP, "demo://") {
		return nil, nil, fmt.Errorf("演示模式：SFTP 功能不可用（需要真实服务器）")
	}

	// 解密敏感字段；解密失败说明加密密钥已变更，禁止拿密文尝试认证（必败且误导）
	if plain, ok := utils.DecryptEx(server.Password); ok {
		server.Password = plain
	} else {
		return nil, nil, fmt.Errorf("服务器密码无法解密（加密密钥已变更），请在服务器列表中重新编辑并保存该服务器密码")
	}
	if plain, ok := utils.DecryptEx(server.PrivateKey); ok {
		server.PrivateKey = plain
	} else {
		return nil, nil, fmt.Errorf("服务器私钥无法解密（加密密钥已变更），请在服务器列表中重新编辑并保存该服务器私钥")
	}
	var authMethods []ssh.AuthMethod
	if server.AuthType == "key" {
		signer, err := ssh.ParsePrivateKey([]byte(server.PrivateKey))
		if err != nil {
			return nil, nil, fmt.Errorf("解析私钥失败: %v", err)
		}
		authMethods = append(authMethods, ssh.PublicKeys(signer))
	} else {
		authMethods = append(authMethods, ssh.Password(server.Password))
		// 键盘交互认证（服务器禁用直接密码认证时的替代方式）
		authMethods = append(authMethods, ssh.KeyboardInteractive(func(name, instruction string, questions []string, echos []bool) ([]string, error) {
			answers := make([]string, len(questions))
			for i := range questions {
				answers[i] = server.Password
			}
			return answers, nil
		}))
	}
	config := &ssh.ClientConfig{
		User:            server.Username,
		Auth:            authMethods,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         10 * time.Second,
	}
	addr := fmt.Sprintf("%s:%d", server.IP, server.Port)
	conn, err := ssh.Dial("tcp", addr, config)
	if err != nil {
		return nil, nil, fmt.Errorf("连接服务器失败: %v", err)
	}
	client, err := sftp.NewClient(conn)
	if err != nil {
		conn.Close()
		return nil, nil, fmt.Errorf("创建SFTP客户端失败: %v", err)
	}
	return client, conn, nil
}

func checkServerPermission(c *gin.Context, serverID uint) (*model.Server, bool) {
	role, _ := c.Get("role")
	userID, _ := c.Get("user_id")
	var server model.Server
	if err := dao.DB.First(&server, serverID).Error; err != nil {
		utils.Fail(c, "服务器不存在")
		return nil, false
	}
	if role != "admin" {
		var cnt int64
		dao.DB.Model(&model.UserServer{}).Where("user_id = ? AND server_id = ?", userID, serverID).Count(&cnt)
		if cnt == 0 {
			utils.Fail(c, "无该服务器访问权限")
			return nil, false
		}
	}
	return &server, true
}

func recordSftpLog(c *gin.Context, server *model.Server, operate, path string) {
	userID, _ := c.Get("user_id")
	username, _ := c.Get("username")
	dao.DB.Create(&model.SftpLog{
		UserID:     userID.(uint),
		Username:   username.(string),
		ServerID:   server.ID,
		ServerName: server.Name,
		Operate:    operate,
		Path:       path,
	})
}

// SftpList 获取文件目录列表
func SftpList(c *gin.Context) {
	serverID := c.Query("server_id")
	path := c.Query("path")
	if path == "" {
		path = "/"
	}
	var sid uint
	fmt.Sscanf(serverID, "%d", &sid)

	server, ok := checkServerPermission(c, sid)
	if !ok {
		return
	}
	// Windows 服务器无 SSH/SFTP，走 SMB 管理共享（见 smb.go）
	if server.OsType == "windows" {
		smbList(c, server, path)
		return
	}
	client, conn, err := getSftpClient(server)
	if err != nil {
		utils.Fail(c, err.Error())
		return
	}
	defer client.Close()
	defer conn.Close()

	entries, err := client.ReadDir(path)
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
	var files []fileInfo
	for _, e := range entries {
		files = append(files, fileInfo{
			Name:    e.Name(),
			IsDir:   e.IsDir(),
			Size:    e.Size(),
			ModTime: e.ModTime(),
			Perm:    e.Mode().String(),
		})
	}
	utils.Success(c, gin.H{"path": path, "files": files})
}

// SftpOperate 文件/文件夹操作（增删改）
func SftpOperate(c *gin.Context) {
	var req struct {
		ServerID    uint   `json:"server_id" binding:"required"`
		Path        string `json:"path"`
		OperateType string `json:"operate_type" binding:"required"` // mkdir/rename/delete
		NewName     string `json:"new_name"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Fail(c, "参数错误")
		return
	}
	server, ok := checkServerPermission(c, req.ServerID)
	if !ok {
		return
	}
	// Windows 服务器无 SSH/SFTP，走 SMB 管理共享（见 smb.go）
	if server.OsType == "windows" {
		smbOperate(c, server, req.Path, req.OperateType, req.NewName)
		return
	}
	client, conn, err := getSftpClient(server)
	if err != nil {
		utils.Fail(c, err.Error())
		return
	}
	defer client.Close()
	defer conn.Close()

	switch req.OperateType {
	case "mkdir":
		err = client.MkdirAll(req.Path)
	case "delete":
		// 删除文件或目录
		info, e := client.Stat(req.Path)
		if e == nil && info.IsDir() {
			err = client.RemoveDirectory(req.Path)
		} else {
			err = client.Remove(req.Path)
		}
	case "rename":
		err = client.Rename(req.Path, path.Join(path.Dir(req.Path), req.NewName))
	default:
		utils.Fail(c, "不支持的操作类型")
		return
	}
	if err != nil {
		utils.Fail(c, "操作失败: "+err.Error())
		return
	}
	recordSftpLog(c, server, req.OperateType, req.Path)
	utils.Success(c, nil)
}

// SftpUpload 文件上传
func SftpUpload(c *gin.Context) {
	serverID := c.PostForm("server_id")
	targetPath := c.PostForm("target_path")
	if targetPath == "" {
		targetPath = "/"
	}
	var sid uint
	fmt.Sscanf(serverID, "%d", &sid)

	server, ok := checkServerPermission(c, sid)
	if !ok {
		return
	}
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		utils.Fail(c, "获取上传文件失败: "+err.Error())
		return
	}
	defer file.Close()

	// Windows 服务器无 SSH/SFTP，走 SMB 管理共享（见 smb.go）
	relativePath := c.PostForm("relative_path")
	if server.OsType == "windows" {
		smbUpload(c, server, targetPath, file, header.Size, relativePath, header.Filename)
		return
	}

	client, conn, err := getSftpClient(server)
	if err != nil {
		utils.Fail(c, err.Error())
		return
	}
	defer client.Close()
	defer conn.Close()

	// 解析目标路径：relative_path 非空时保留文件夹目录结构（文件夹上传），否则用单文件名
	var dstPath string
	if relativePath != "" {
		// 清洗：统一为 POSIX 路径，去掉前导分隔符，禁止 .. 越权跳出 targetPath
		rel := path.Clean(strings.ReplaceAll(strings.TrimPrefix(relativePath, "/"), "\\", "/"))
		if rel == "." || rel == ".." || strings.HasPrefix(rel, "../") {
			utils.Fail(c, "相对路径非法")
			return
		}
		dstPath = path.Join(targetPath, rel)
	} else {
		// 远程一律为 POSIX 路径；path.Base 去掉可能夹带的路径分隔符，防止写到非预期目录
		safeName := path.Base(header.Filename)
		if safeName == "." || safeName == "/" || safeName == "\\" || safeName == "" {
			utils.Fail(c, "文件名非法")
			return
		}
		dstPath = path.Join(targetPath, safeName)
	}
	// 文件夹上传场景：先递归创建父目录，避免 Create 因目录不存在失败
	if dir := path.Dir(dstPath); dir != "." && dir != "/" {
		if err := client.MkdirAll(dir); err != nil {
			utils.Fail(c, "创建远程目录失败: "+err.Error())
			return
		}
	}
	dst, err := client.Create(dstPath)
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
	// 强制刷盘并核对字节数，确保服务端确实完整写入，避免“假成功”
	if err = dst.Sync(); err != nil {
		dst.Close()
		utils.Fail(c, "文件落盘失败: "+err.Error())
		return
	}
	dst.Close()

	if written != header.Size {
		utils.Fail(c, fmt.Sprintf("上传不完整: 已发送 %d 字节，应为 %d 字节", written, header.Size))
		return
	}
	if info, e := client.Stat(dstPath); e == nil && info.Size() != header.Size {
		utils.Fail(c, fmt.Sprintf("服务器文件大小校验失败: 实际 %d 字节，应为 %d 字节", info.Size(), header.Size))
		return
	}
	recordSftpLog(c, server, "upload", dstPath)
	utils.Success(c, gin.H{"path": dstPath, "name": path.Base(dstPath), "size": header.Size})
}

// SftpDownload 文件下载
func SftpDownload(c *gin.Context) {
	serverID := c.Query("server_id")
	filePath := c.Query("file_path")
	var sid uint
	fmt.Sscanf(serverID, "%d", &sid)

	server, ok := checkServerPermission(c, sid)
	if !ok {
		return
	}
	// Windows 服务器无 SSH/SFTP，走 SMB 管理共享（见 smb.go）
	if server.OsType == "windows" {
		smbDownload(c, server, filePath)
		return
	}
	client, conn, err := getSftpClient(server)
	if err != nil {
		utils.Fail(c, err.Error())
		return
	}
	defer client.Close()
	defer conn.Close()

	src, err := client.Open(filePath)
	if err != nil {
		utils.Fail(c, "打开远程文件失败: "+err.Error())
		return
	}
	defer src.Close()

	recordSftpLog(c, server, "download", filePath)
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", path.Base(filePath)))
	c.Header("Content-Type", "application/octet-stream")
	io.Copy(c.Writer, src)
}

// 确保sshmgr包被引用（用于在线会话复用等）
var _ = sshmgr.GetSession
var _ = os.Create
