package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"qinjiagateway/utils"

	"github.com/gin-gonic/gin"
)

// AIChat AI助手代理接口（免费供应商：智谱GLM-4-Flash / SiliconFlow免费模型 / Ollama本地）
// 后端统一转发：规避浏览器CORS、隐藏调用细节；API Key由前端保存并随请求传入
func AIChat(c *gin.Context) {
	var req struct {
		Provider string `json:"provider"` // zhipu | siliconflow | ollama
		APIKey   string `json:"api_key"`
		Model    string `json:"model"`
		Messages []struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"messages"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Fail(c, "参数错误")
		return
	}
	if len(req.Messages) == 0 {
		utils.Fail(c, "消息不能为空")
		return
	}

	// 上游地址与Key要求
	var endpoint string
	var needKey bool
	switch req.Provider {
	case "zhipu": // 智谱 GLM-4-Flash 免费
		endpoint = "https://open.bigmodel.cn/api/paas/v4/chat/completions"
		needKey = true
		if req.Model == "" {
			req.Model = "glm-4-flash"
		}
	case "siliconflow": // SiliconFlow 免费模型（如 Qwen/Qwen2.5-7B-Instruct 免费版）
		endpoint = "https://api.siliconflow.cn/v1/chat/completions"
		needKey = true
		if req.Model == "" {
			req.Model = "Qwen/Qwen2.5-7B-Instruct"
		}
	case "ollama": // 本地 Ollama（OpenAI兼容端点），完全免费
		endpoint = "http://localhost:11434/v1/chat/completions"
		if req.Model == "" {
			req.Model = "qwen2.5:7b"
		}
	default:
		utils.Fail(c, "不支持的AI供应商")
		return
	}
	if needKey && strings.TrimSpace(req.APIKey) == "" {
		utils.Fail(c, "请先在设置中填写该供应商的API Key")
		return
	}

	// 组装消息：内置系统提示词（操作系统级技术支持专家）
	type chatMsg struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}
	msgs := make([]chatMsg, 0, len(req.Messages)+1)
	msgs = append(msgs, chatMsg{Role: "system", Content: systemPrompt})
	for _, m := range req.Messages {
		role := m.Role
		if role != "user" && role != "assistant" {
			continue
		}
		content := strings.TrimSpace(m.Content)
		if content == "" {
			continue
		}
		msgs = append(msgs, chatMsg{Role: role, Content: content})
	}
	if len(msgs) == 1 {
		utils.Fail(c, "消息内容不能为空")
		return
	}

	payload, _ := json.Marshal(gin.H{
		"model":    req.Model,
		"messages": msgs,
		"stream":   false,
	})

	httpReq, err := http.NewRequest("POST", endpoint, bytes.NewReader(payload))
	if err != nil {
		utils.Fail(c, "创建请求失败")
		return
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if req.APIKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+req.APIKey)
	}

	client := &http.Client{Timeout: 90 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		utils.Fail(c, "AI服务连接失败: "+err.Error())
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode != 200 {
		utils.Fail(c, fmt.Sprintf("AI服务返回 %d: %s", resp.StatusCode, truncate(string(body), 300)))
		return
	}

	// 统一解析 OpenAI 兼容响应
	var out struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(body, &out); err != nil || len(out.Choices) == 0 {
		utils.Fail(c, "AI响应解析失败: "+truncate(string(body), 300))
		return
	}
	content := out.Choices[0].Message.Content
	if strings.TrimSpace(content) == "" {
		utils.Fail(c, "AI返回内容为空")
		return
	}
	utils.Success(c, gin.H{"content": content, "model": req.Model})
}

const systemPrompt = `你是「勤加软件WEB服务器运维管理平台」内置的AI运维助手，专精于操作系统级别的技术支持，特别是Linux服务器的运维管理。

你的职责：
1. 解答操作系统相关问题：Linux（CentOS/Ubuntu/Debian等）命令使用、Shell脚本、系统服务、进程管理、磁盘/内存/CPU排查、网络配置、防火墙、日志分析、性能调优、故障诊断等。
2. 也兼顾 Windows 服务器、Nginx/Docker/数据库等常见服务软件的运维问题。
3. 给出可直接执行的命令或配置片段，并解释关键参数与风险点；危险操作（如rm -rf、格式化、重启服务）必须明确警告。
4. 排查类问题按「先看什么→命令是什么→结果怎么判断→下一步怎么办」的结构化思路回答。
5. 回答使用简体中文，简洁准确，代码块用markdown标注语言。如用户提供了终端输出上下文，优先基于上下文分析。`

func truncate(s string, n int) string {
	r := []rune(strings.TrimSpace(s))
	if len(r) <= n {
		return string(r)
	}
	return string(r[:n]) + "..."
}
