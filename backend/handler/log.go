package handler

import (
	"fmt"
	"time"

	"qinjiagateway/dao"
	"qinjiagateway/model"
	sshmgr "qinjiagateway/ssh"
	"qinjiagateway/utils"

	"github.com/gin-gonic/gin"
)

// LogLogin 登录日志列表
func LogLogin(c *gin.Context) {
	var logs []model.LoginLog
	page, limit := getPage(c)
	query := dao.DB.Model(&model.LoginLog{})
	if st := c.Query("start_time"); st != "" {
		query = query.Where("created_at >= ?", st)
	}
	if et := c.Query("end_time"); et != "" {
		query = query.Where("created_at <= ?", et)
	}
	var total int64
	query.Count(&total)
	query.Order("id DESC").Offset((page - 1) * limit).Limit(limit).Find(&logs)
	utils.Success(c, gin.H{"list": logs, "total": total})
}

// LogCommand 终端命令日志
func LogCommand(c *gin.Context) {
	var logs []model.CommandLog
	page, limit := getPage(c)
	query := dao.DB.Model(&model.CommandLog{})
	if st := c.Query("start_time"); st != "" {
		query = query.Where("created_at >= ?", st)
	}
	if et := c.Query("end_time"); et != "" {
		query = query.Where("created_at <= ?", et)
	}
	var total int64
	query.Count(&total)
	query.Order("id DESC").Offset((page - 1) * limit).Limit(limit).Find(&logs)
	utils.Success(c, gin.H{"list": logs, "total": total})
}

// LogSftp 文件操作日志
func LogSftp(c *gin.Context) {
	var logs []model.SftpLog
	page, limit := getPage(c)
	query := dao.DB.Model(&model.SftpLog{})
	if st := c.Query("start_time"); st != "" {
		query = query.Where("created_at >= ?", st)
	}
	if et := c.Query("end_time"); et != "" {
		query = query.Where("created_at <= ?", et)
	}
	var total int64
	query.Count(&total)
	query.Order("id DESC").Offset((page - 1) * limit).Limit(limit).Find(&logs)
	utils.Success(c, gin.H{"list": logs, "total": total})
}

// LogExport 日志导出（CSV）
func LogExport(c *gin.Context) {
	logType := c.Query("log_type")
	startTime := c.Query("start_time")
	endTime := c.Query("end_time")

	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s_logs.csv", logType))

	var csvData string
	switch logType {
	case "login":
		csvData = "用户名,登录IP,设备信息,状态,登录时间\n"
		var logs []model.LoginLog
		q := dao.DB.Model(&model.LoginLog{})
		if startTime != "" {
			q = q.Where("created_at >= ?", startTime)
		}
		if endTime != "" {
			q = q.Where("created_at <= ?", endTime)
		}
		q.Find(&logs)
		for _, l := range logs {
			status := "失败"
			if l.Status == 1 {
				status = "成功"
			}
			csvData += fmt.Sprintf("%s,%s,%s,%s,%s\n", l.Username, l.IP, l.UserAgent, status, l.CreatedAt.Format("2006-01-02 15:04:05"))
		}
	case "command":
		csvData = "用户名,服务器,执行命令,执行时间\n"
		var logs []model.CommandLog
		q := dao.DB.Model(&model.CommandLog{})
		if startTime != "" {
			q = q.Where("created_at >= ?", startTime)
		}
		if endTime != "" {
			q = q.Where("created_at <= ?", endTime)
		}
		q.Find(&logs)
		for _, l := range logs {
			csvData += fmt.Sprintf("%s,%s,%q,%s\n", l.Username, l.ServerName, l.Command, l.CreatedAt.Format("2006-01-02 15:04:05"))
		}
	case "sftp":
		csvData = "用户名,服务器,操作类型,文件路径,操作时间\n"
		var logs []model.SftpLog
		q := dao.DB.Model(&model.SftpLog{})
		if startTime != "" {
			q = q.Where("created_at >= ?", startTime)
		}
		if endTime != "" {
			q = q.Where("created_at <= ?", endTime)
		}
		q.Find(&logs)
		for _, l := range logs {
			csvData += fmt.Sprintf("%s,%s,%s,%s,%s\n", l.Username, l.ServerName, l.Operate, l.Path, l.CreatedAt.Format("2006-01-02 15:04:05"))
		}
	default:
		utils.Fail(c, "不支持的日志类型")
		return
	}
	c.Writer.Write([]byte("\xEF\xBB\xBF")) // BOM头解决Excel中文乱码
	c.Writer.Write([]byte(csvData))
}

func getPage(c *gin.Context) (int, int) {
	page := 1
	limit := 20
	if p := c.Query("page"); p != "" {
		fmt.Sscanf(p, "%d", &page)
	}
	if l := c.Query("limit"); l != "" {
		fmt.Sscanf(l, "%d", &limit)
	}
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}
	return page, limit
}

// Dashboard 概览统计
func Dashboard(c *gin.Context) {
	var userCount, serverCount int64
	dao.DB.Model(&model.User{}).Count(&userCount)
	dao.DB.Model(&model.Server{}).Count(&serverCount)
	onlineSession := int64(len(sshmgr.GetAllSessions()))
	var todayLogin int64
	today := time.Now().Format("2006-01-02")
	dao.DB.Model(&model.LoginLog{}).Where("created_at >= ? AND status = 1", today).Count(&todayLogin)
	utils.Success(c, gin.H{
		"user_count":     userCount,
		"server_count":   serverCount,
		"online_session": onlineSession,
		"today_login":    todayLogin,
	})
}
