package monitor

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"qinjiagateway/demo"
	"qinjiagateway/model"

	"gorm.io/gorm"
)

// UrlStatus 单个网址的最新监控状态
type UrlStatus struct {
	UrlID        uint       `json:"url_id"`
	ServerID     uint       `json:"server_id"`
	Url          string     `json:"url"`
	Protocol     string     `json:"protocol"`
	Domain       string     `json:"domain"`
	Accessible   bool       `json:"accessible"`
	StatusCode   int        `json:"status_code"`
	LatencyMs    int64      `json:"latency_ms"`
	CertIssuer   string     `json:"cert_issuer"`
	CertExpireAt *time.Time `json:"cert_expire_at"`
	CertDaysLeft int        `json:"cert_days_left"`
	CertValid    bool       `json:"cert_valid"`
	CertError    string     `json:"cert_error"`
	Error        string     `json:"error"`
	CheckedAt    time.Time  `json:"checked_at"`
}

var (
	mu       sync.RWMutex
	results  = make(map[uint]*UrlStatus) // key: url_id
	checking int32
)

// StartBackground 启动后台定时监控
func StartBackground(db *gorm.DB, interval time.Duration) {
	go func() {
		CheckAll(db, true)
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for range ticker.C {
			CheckAll(db, false)
		}
	}()
}

// CheckAll 检测所有网址（async=true时异步执行）
func CheckAll(db *gorm.DB, async bool) {
	if async {
		go checkAll(db)
	} else {
		checkAll(db)
	}
}

func checkAll(db *gorm.DB) {
	// 防止重入（上一轮未跑完时跳过）
	if !atomic.CompareAndSwapInt32(&checking, 0, 1) {
		return
	}
	defer atomic.StoreInt32(&checking, 0)

	var urls []model.ServerUrl
	db.Find(&urls)

	var wg sync.WaitGroup
	for _, u := range urls {
		wg.Add(1)
		go func(u model.ServerUrl) {
			defer wg.Done()
			st := CheckOne(u)
			mu.Lock()
			results[u.ID] = st
			mu.Unlock()
		}(u)
	}
	wg.Wait()
}

// CheckOne 检测单个网址：访问延时、状态码、HTTPS证书状态
func CheckOne(u model.ServerUrl) *UrlStatus {
	// 演示模式：模拟 URL 直接合成结果
	if demo.IsDemoDomain(u.Domain) {
		return simulateCheck(u)
	}

	st := &UrlStatus{
		UrlID:      u.ID,
		ServerID:   u.ServerID,
		Protocol:   u.Protocol,
		Domain:     u.Domain,
		CheckedAt:  time.Now(),
		Accessible: false,
	}
	fullURL, _ := BuildURL(&u)
	st.Url = fullURL

	client := &http.Client{
		Timeout: 8 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true, ServerName: u.Domain},
		},
	}
	start := time.Now()
	resp, err := client.Get(fullURL)
	st.LatencyMs = time.Since(start).Milliseconds()
	if err != nil {
		st.Error = err.Error()
		return st
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))

	st.Accessible = true
	st.StatusCode = resp.StatusCode

	// HTTPS证书信息
	if resp.TLS != nil && len(resp.TLS.PeerCertificates) > 0 {
		cert := resp.TLS.PeerCertificates[0]
		st.CertIssuer = cert.Issuer.CommonName
		expire := cert.NotAfter
		st.CertExpireAt = &expire
		st.CertDaysLeft = int(time.Until(expire).Hours() / 24)

		// 完整证书校验：有效期窗口 + 域名匹配 + 证书链信任
		// （HTTP 探测用 InsecureSkipVerify 以保证站点仍可访问，
		//   因此证书问题必须在此显式校验，不能依赖握手结果）
		now := time.Now()
		var problems []string
		if now.Before(cert.NotBefore) {
			problems = append(problems, "证书尚未生效")
		}
		if now.After(cert.NotAfter) {
			problems = append(problems, "证书已过期")
		}
		// 域名匹配：CN/SAN 必须包含被监控域名
		if err := cert.VerifyHostname(u.Domain); err != nil {
			problems = append(problems, "证书域名不匹配（CN/SAN 不含 "+u.Domain+"）")
		}
		// 证书链：用系统根证书 + 服务器下发的中间证书验证
		opts := x509.VerifyOptions{
			DNSName:       u.Domain,
			Intermediates: x509.NewCertPool(),
			CurrentTime:   now,
		}
		for _, ic := range resp.TLS.PeerCertificates[1:] {
			opts.Intermediates.AddCert(ic)
		}
		if _, err := cert.Verify(opts); err != nil {
			// 域名问题与有效期问题已单独给出明确提示，这里只补充链级问题，避免重复
			var invalidErr x509.CertificateInvalidError
			var unknownAuth x509.UnknownAuthorityError
			switch {
			case errors.As(err, &invalidErr) && invalidErr.Reason == x509.Expired:
				// 已过期，上面已记录
			case errors.As(err, &unknownAuth):
				problems = append(problems, "证书链不受信任（自签名或缺少中间证书）")
			default:
				if len(problems) == 0 {
					problems = append(problems, "证书校验失败："+err.Error())
				}
			}
		}
		if len(problems) == 0 {
			st.CertValid = true
		} else {
			st.CertValid = false
			st.CertError = strings.Join(problems, "；")
		}
	}
	return st
}

// BuildURL 拼接完整URL和地址，端口为0时使用协议默认端口
func BuildURL(u *model.ServerUrl) (fullURL, addr string) {
	port := u.Port
	if port == 0 {
		if u.Protocol == "http" {
			port = 80
		} else {
			port = 443
		}
	}
	isDefault := (u.Protocol == "https" && port == 443) || (u.Protocol == "http" && port == 80)
	host := u.Domain
	if !isDefault {
		host = fmt.Sprintf("%s:%d", u.Domain, port)
	}
	return fmt.Sprintf("%s://%s", u.Protocol, host), fmt.Sprintf("%s:%d", u.Domain, port)
}

// GetResults 获取全部监控结果
func GetResults() map[uint]*UrlStatus {
	mu.RLock()
	defer mu.RUnlock()
	copyMap := make(map[uint]*UrlStatus, len(results))
	for k, v := range results {
		copyMap[k] = v
	}
	return copyMap
}

// simulateCheck 演示模式：根据域名尾缀合成监控结果
func simulateCheck(u model.ServerUrl) *UrlStatus {
	st := &UrlStatus{
		UrlID:      u.ID,
		ServerID:   u.ServerID,
		Protocol:   u.Protocol,
		Domain:     u.Domain,
		CheckedAt:  time.Now(),
		Accessible: false,
	}
	fullURL, _ := BuildURL(&u)
	st.Url = fullURL

	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	now := time.Now()

	// 根据域名前缀返回预设场景
	switch {
	case strings.HasPrefix(u.Domain, "ok-180."):
		// 正常：200 + 证书 180 天
		st.Accessible = true
		st.StatusCode = 200
		st.LatencyMs = int64(20 + rng.Intn(30))
		st.CertIssuer = "Let's Encrypt"
		expire := now.AddDate(0, 6, 0) // 180 天后
		st.CertExpireAt = &expire
		st.CertDaysLeft = 180
		st.CertValid = true

	case strings.HasPrefix(u.Domain, "warn-25."):
		// 证书临期：200 + 证书 25 天
		st.Accessible = true
		st.StatusCode = 200
		st.LatencyMs = int64(30 + rng.Intn(40))
		st.CertIssuer = "Let's Encrypt"
		expire := now.AddDate(0, 0, 25) // 25 天后
		st.CertExpireAt = &expire
		st.CertDaysLeft = 25
		st.CertValid = true

	case strings.HasPrefix(u.Domain, "expired."):
		// 证书过期：200 + 证书已过期
		st.Accessible = true
		st.StatusCode = 200
		st.LatencyMs = int64(50 + rng.Intn(50))
		st.CertIssuer = "Let's Encrypt"
		expire := now.AddDate(0, 0, -5) // 5 天前过期
		st.CertExpireAt = &expire
		st.CertDaysLeft = -5
		st.CertValid = false
		st.CertError = "证书已过期"

	case strings.HasPrefix(u.Domain, "http200."):
		// 纯 HTTP：200 无证书
		st.Accessible = true
		st.StatusCode = 200
		st.LatencyMs = int64(15 + rng.Intn(20))

	case strings.HasPrefix(u.Domain, "err503."):
		// HTTP 503 错误
		st.Accessible = true
		st.StatusCode = 503
		st.LatencyMs = int64(100 + rng.Intn(100))
		st.Error = "HTTP 503 Service Unavailable"

	case strings.HasPrefix(u.Domain, "timeout."):
		// 连接超时
		st.Accessible = false
		st.LatencyMs = 8000
		st.Error = "连接超时（8秒）"

	default:
		// 未知演示域名：默认正常
		st.Accessible = true
		st.StatusCode = 200
		st.LatencyMs = int64(25 + rng.Intn(25))
		if u.Protocol == "https" {
			st.CertIssuer = "Let's Encrypt"
			expire := now.AddDate(0, 3, 0) // 90 天后
			st.CertExpireAt = &expire
			st.CertDaysLeft = 90
			st.CertValid = true
		}
	}

	return st
}
