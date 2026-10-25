package notify

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/smtp"
	"strings"
	"sync"
	"time"

	"qinjiagateway/dao"
	"qinjiagateway/model"
	"qinjiagateway/utils"
)

// sensitiveParamKeys 需要加密存储的敏感参数（邮件授权码、钉钉 Token/AppSecret 等）。
// LoadConfig 时解密，Save 时加密，GET 接口返回时掩码。
var sensitiveParamKeys = map[string]bool{
	"pass":       true, // 邮件 SMTP 授权码
	"token":      true, // 钉钉自定义机器人 access_token
	"app_secret": true, // 钉钉应用机器人 AppSecret
	"key":        true, // 企业微信 Webhook key
}

// Channel 单个通道的配置
type Channel struct {
	Enabled bool                   `json:"enabled"`
	Params  map[string]interface{} `json:"params"`
}

// Config 告警通知整体配置
type Config struct {
	WeCom    *Channel `json:"wecom"`
	DingTalk *Channel `json:"dingtalk"`
	Email    *Channel `json:"email"`
	SMS      *Channel `json:"sms"`
}

// LoadConfig 从数据库读取告警配置；不存在则返回全禁用的默认配置
func LoadConfig() (*Config, error) {
	var set model.AlertSetting
	err := dao.DB.Order("id ASC").First(&set).Error
	if err != nil {
		return defaultConfig(), nil
	}
	cfg := &Config{}
	if err := json.Unmarshal([]byte(set.Channels), cfg); err != nil {
		return defaultConfig(), nil
	}
	// 解密各通道中的敏感参数（兼容旧明文：utils.Decrypt 对无前缀值原样返回）
	decryptSensitive(cfg.WeCom)
	decryptSensitive(cfg.DingTalk)
	decryptSensitive(cfg.Email)
	decryptSensitive(cfg.SMS)
	return cfg, nil
}

// decryptSensitive 解密通道参数中的敏感字段；空值或非密文原样返回
func decryptSensitive(ch *Channel) {
	if ch == nil {
		return
	}
	for k, v := range ch.Params {
		if !sensitiveParamKeys[k] {
			continue
		}
		if s, ok := v.(string); ok {
			// 解密失败（密钥已变更）：置空并提示重新配置，避免密文被当作真实凭据外发
			if plain, decOK := utils.DecryptEx(s); decOK {
				ch.Params[k] = plain
			} else {
				log.Printf("[notify] 通道参数 %s 解密失败（加密密钥已变更），请在通知设置中重新配置", k)
				ch.Params[k] = ""
			}
		}
	}
}

// maskSensitive 将敏感参数替换为掩码 "******"，避免回传明文到前端
func maskSensitive(ch *Channel) {
	if ch == nil {
		return
	}
	for k := range ch.Params {
		if sensitiveParamKeys[k] {
			if s, ok := ch.Params[k].(string); ok && s != "" {
				ch.Params[k] = "******"
			}
		}
	}
}

// ForDisplay 返回一份深拷贝的配置，敏感字段已替换为掩码，供 GET 接口回传前端。
// 不修改原配置对象（原对象保留明文，供实际发送通知使用）。
func (c *Config) ForDisplay() *Config {
	data, _ := json.Marshal(c)
	cp := &Config{}
	_ = json.Unmarshal(data, cp)
	maskSensitive(cp.WeCom)
	maskSensitive(cp.DingTalk)
	maskSensitive(cp.Email)
	maskSensitive(cp.SMS)
	return cp
}

// EncryptForSave 加密各通道敏感字段后序列化为 JSON 字符串，供保存接口写入数据库
func (c *Config) EncryptForSave() (string, error) {
	encryptSensitive := func(ch *Channel) {
		if ch == nil {
			return
		}
		for k, v := range ch.Params {
			if !sensitiveParamKeys[k] {
				continue
			}
			if s, ok := v.(string); ok {
				ch.Params[k] = utils.Encrypt(s)
			}
		}
	}
	encryptSensitive(c.WeCom)
	encryptSensitive(c.DingTalk)
	encryptSensitive(c.Email)
	encryptSensitive(c.SMS)
	data, err := json.Marshal(c)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// MaskForEmptyUpdate 处理保存时的空敏感字段：若某敏感字段值为掩码或空，
// 则从已存配置中取出原值保留（前端留空表示不修改）。
// oldCfg 为数据库中已解密的旧配置。
func (c *Config) MaskForEmptyUpdate(oldCfg *Config) {
	merge := func(cur, old *Channel) {
		if cur == nil || old == nil {
			return
		}
		for k := range cur.Params {
			if !sensitiveParamKeys[k] {
				continue
			}
			s, _ := cur.Params[k].(string)
			if s == "" || s == "******" {
				if oldS, ok := old.Params[k].(string); ok {
					cur.Params[k] = oldS
				}
			}
		}
	}
	merge(c.WeCom, oldCfg.WeCom)
	merge(c.DingTalk, oldCfg.DingTalk)
	merge(c.Email, oldCfg.Email)
	merge(c.SMS, oldCfg.SMS)
}

func defaultConfig() *Config {
	return &Config{
		WeCom:    &Channel{Enabled: false, Params: map[string]interface{}{}},
		DingTalk: &Channel{Enabled: false, Params: map[string]interface{}{}},
		Email:    &Channel{Enabled: false, Params: map[string]interface{}{}},
		SMS:      &Channel{Enabled: false, Params: map[string]interface{}{}},
	}
}

func strParam(p map[string]interface{}, key string) string {
	v, ok := p[key]
	if !ok {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return fmt.Sprintf("%v", v)
}

// SendAlert 向所有已启用通道发送告警；返回各通道结果摘要
func SendAlert(title, content string) map[string]string {
	cfg, err := LoadConfig()
	if err != nil {
		return map[string]string{"error": err.Error()}
	}
	results := map[string]string{}
	if cfg.WeCom != nil && cfg.WeCom.Enabled {
		if e := sendWeCom(cfg.WeCom.Params, title, content); e != nil {
			results["wecom"] = "失败: " + e.Error()
		} else {
			results["wecom"] = "成功"
		}
	}
	if cfg.DingTalk != nil && cfg.DingTalk.Enabled {
		if e := sendDingTalk(cfg.DingTalk.Params, title, content); e != nil {
			results["dingtalk"] = "失败: " + e.Error()
		} else {
			results["dingtalk"] = "成功"
		}
	}
	if cfg.Email != nil && cfg.Email.Enabled {
		if e := sendEmail(cfg.Email.Params, title, content); e != nil {
			results["email"] = "失败: " + e.Error()
		} else {
			results["email"] = "成功"
		}
	}
	if cfg.SMS != nil && cfg.SMS.Enabled {
		if e := sendSMS(cfg.SMS.Params, title, content); e != nil {
			results["sms"] = "失败: " + e.Error()
		} else {
			results["sms"] = "成功"
		}
	}
	return results
}

// SendTest 向指定通道发送一条测试消息
func SendTest(channel string, params map[string]interface{}) error {
	title := "【测试告警】"
	content := fmt.Sprintf("这是一条来自运维网关的测试消息，发送时间：%s", time.Now().Format("2006-01-02 15:04:05"))
	switch channel {
	case "wecom":
		return sendWeCom(params, title, content)
	case "dingtalk":
		return sendDingTalk(params, title, content)
	case "email":
		return sendEmail(params, title, content)
	case "sms":
		return sendSMS(params, title, content)
	default:
		return fmt.Errorf("未知通道: %s", channel)
	}
}

var httpClient = &http.Client{Timeout: 15 * time.Second}

func postJSON(url string, body interface{}) error {
	data, _ := json.Marshal(body)
	resp, err := httpClient.Post(url, "application/json", bytes.NewReader(data))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	// 解析返回体中的 errcode（企微/钉钉通用）
	var result map[string]interface{}
	if json.NewDecoder(resp.Body).Decode(&result) == nil {
		if code, ok := result["errcode"]; ok {
			if f, ok := code.(float64); ok && f != 0 {
				msg, _ := result["errmsg"].(string)
				return fmt.Errorf("errcode=%v, errmsg=%s", f, msg)
			}
		}
	}
	return nil
}

// 企业微信群机器人
func sendWeCom(p map[string]interface{}, title, content string) error {
	key := strParam(p, "key")
	if key == "" {
		return fmt.Errorf("缺少 Webhook Key")
	}
	url := "https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=" + key
	msg := map[string]interface{}{
		"msgtype": "markdown",
		"markdown": map[string]string{
			"content": fmt.Sprintf("### %s\n%s", title, content),
		},
	}
	return postJSON(url, msg)
}

// 钉钉群机器人（两种模式）：
//   - webhook：自定义群机器人（params: token）
//   - robot：  企业内部应用机器人（params: app_key/app_secret/robot_code/conversation_id），
//     通过 RobotCode 调用官方 API 发送
func sendDingTalk(p map[string]interface{}, title, content string) error {
	if strParam(p, "mode") == "robot" || strParam(p, "robot_code") != "" {
		return sendDingTalkRobot(p, title, content)
	}
	token := strParam(p, "token")
	if token == "" {
		return fmt.Errorf("缺少 Access Token")
	}
	url := "https://oapi.dingtalk.com/robot/send?access_token=" + token
	msg := map[string]interface{}{
		"msgtype": "markdown",
		"markdown": map[string]string{
			"title": title,
			"text":  fmt.Sprintf("### %s\n%s", title, content),
		},
	}
	return postJSON(url, msg)
}

// ---- 钉钉企业内部应用机器人（RobotCode）----

var dingTokenCache = struct {
	sync.Mutex
	appKey, token string
	expireAt      time.Time
}{}

// getDingTalkAccessToken 使用 AppKey/AppSecret 换取应用 accessToken（缓存至到期前 5 分钟）
func getDingTalkAccessToken(appKey, appSecret string) (string, error) {
	dingTokenCache.Lock()
	defer dingTokenCache.Unlock()
	if dingTokenCache.appKey == appKey && time.Now().Before(dingTokenCache.expireAt) {
		return dingTokenCache.token, nil
	}
	data, _ := json.Marshal(map[string]string{"appKey": appKey, "appSecret": appSecret})
	resp, err := httpClient.Post("https://api.dingtalk.com/v1.0/oauth2/accessToken",
		"application/json", bytes.NewReader(data))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	var r struct {
		AccessToken string `json:"accessToken"`
		ExpireIn    int    `json:"expireIn"`
		Code        string `json:"code"`
		Message     string `json:"message"`
	}
	body, _ := io.ReadAll(resp.Body)
	if err := json.Unmarshal(body, &r); err != nil {
		return "", fmt.Errorf("token 响应解析失败: HTTP %d", resp.StatusCode)
	}
	if r.AccessToken == "" {
		return "", fmt.Errorf("获取 accessToken 失败: code=%s, message=%s", r.Code, r.Message)
	}
	dingTokenCache.appKey = appKey
	dingTokenCache.token = r.AccessToken
	dingTokenCache.expireAt = time.Now().Add(time.Duration(r.ExpireIn)*time.Second - 5*time.Minute)
	return r.AccessToken, nil
}

// sendDingTalkRobot 通过 RobotCode 调用应用机器人接口发送 Markdown 群消息
func sendDingTalkRobot(p map[string]interface{}, title, content string) error {
	appKey := strParam(p, "app_key")
	appSecret := strParam(p, "app_secret")
	robotCode := strParam(p, "robot_code")
	conversationID := strParam(p, "conversation_id")
	if appKey == "" || appSecret == "" || robotCode == "" || conversationID == "" {
		return fmt.Errorf("缺少 AppKey/AppSecret/RobotCode/会话ID")
	}
	accessToken, err := getDingTalkAccessToken(appKey, appSecret)
	if err != nil {
		return err
	}

	// msgParam 须为 JSON 字符串（sampleMarkdown 模板：title + text）
	msgParam, _ := json.Marshal(map[string]string{
		"title": title,
		"text":  fmt.Sprintf("### %s\n%s", title, content),
	})
	reqBody, _ := json.Marshal(map[string]string{
		"robotCode":          robotCode,
		"openConversationId": conversationID,
		"msgKey":             "sampleMarkdown",
		"msgParam":           string(msgParam),
	})

	req, err := http.NewRequest(http.MethodPost,
		"https://api.dingtalk.com/v1.0/robot/groupMessages/send", bytes.NewReader(reqBody))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-acs-dingtalk-access-token", accessToken)
	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	respBytes, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		// 新 API 错误格式：{"code":"...","message":"..."}
		var e struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		}
		json.Unmarshal(respBytes, &e)
		return fmt.Errorf("钉钉API错误 HTTP %d: code=%s, message=%s",
			resp.StatusCode, e.Code, e.Message)
	}
	return nil
}

// 邮件（SMTP）
func sendEmail(p map[string]interface{}, title, content string) error {
	host := strParam(p, "host")
	port := strParam(p, "port")
	user := strParam(p, "user")
	pass := strParam(p, "pass")
	to := strParam(p, "to")
	if host == "" || user == "" || to == "" {
		return fmt.Errorf("主机/账号/收件人不能为空")
	}
	if port == "" {
		port = "25"
	}
	addr := host + ":" + port
	body := "From: " + user + "\r\n" +
		"To: " + to + "\r\n" +
		"Subject: " + title + "\r\n" +
		"MIME-Version: 1.0\r\n" +
		"Content-Type: text/plain; charset=UTF-8\r\n\r\n" +
		content
	useTLS := port == "465" || strParam(p, "ssl") == "true"
	if useTLS {
		tlsCfg := &tls.Config{ServerName: host}
		conn, err := tls.Dial("tcp", addr, tlsCfg)
		if err != nil {
			return err
		}
		c, err := smtp.NewClient(conn, host)
		if err != nil {
			return err
		}
		defer c.Close()
		if err = c.Auth(smtp.PlainAuth("", user, pass, host)); err != nil {
			return err
		}
		if err = c.Mail(user); err != nil {
			return err
		}
		for _, rcpt := range strings.Split(to, ",") {
			if err = c.Rcpt(strings.TrimSpace(rcpt)); err != nil {
				return err
			}
		}
		w, err := c.Data()
		if err != nil {
			return err
		}
		if _, err = w.Write([]byte(body)); err != nil {
			return err
		}
		return w.Close()
	}
	auth := smtp.PlainAuth("", user, pass, host)
	return smtp.SendMail(addr, auth, user, strings.Split(to, ","), []byte(body))
}

// 短信（通用 HTTP 网关）
func sendSMS(p map[string]interface{}, title, content string) error {
	url := strParam(p, "url")
	template := strParam(p, "template")
	mobile := strParam(p, "mobile")
	if url == "" || mobile == "" {
		return fmt.Errorf("网关地址/手机号不能为空")
	}
	text := title + " " + content
	body := map[string]interface{}{
		"mobile":   mobile,
		"content":  text,
		"template": template,
	}
	for k, v := range p {
		if k != "url" && k != "template" && k != "mobile" {
			body[k] = v
		}
	}
	return postJSON(url, body)
}
