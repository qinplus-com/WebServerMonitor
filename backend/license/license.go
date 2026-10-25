package license

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdsa"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"log"
	"math/big"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"

	"qinjiagateway/config"
	"qinjiagateway/dao"
	"qinjiagateway/model"
	"qinjiagateway/notify"
)

// Edition 当前版本信息
type Edition struct {
	Edition    string `json:"edition"`     // personal | sme | enterprise
	MaxServers int    `json:"max_servers"` // -1 表示不限
	ExpireAt   string `json:"expire_at"`   // 到期日期（空表示无限期）
	IssuedTo   string `json:"issued_to"`   // 被授权方
	IsTrial    bool   `json:"is_trial"`    // 是否在试用期
	DaysLeft   int    `json:"days_left"`   // 剩余天数（-1 表示无限期）
}

// 版本服务器上限：个人版 3 台，中小企业板 10 台，企业版不限
const (
	PersonalMaxServers = 3
	SMEMaxServers      = 10
)

var currentEdition *Edition

// downgradeNotice 启动重验失败的原因（数据疑似复制自其他机器/被篡改），
// 供 CheckStartupLimit 弹窗提示；每次 Init 重置
var downgradeNotice string

// Init 初始化授权系统：启动时调用，确定当前版本
// 优先级：DB 中的有效 License > config.yaml 的 edition 设置
// 这样用户激活企业版后，重启服务仍保持企业版（不依赖配置文件）
func Init(cfgEdition string) {
	downgradeNotice = ""
	// 社区版（开源构建，-tags opensource）：跳过授权校验，
	// 无服务器数量与使用期限限制，遵循 Apache-2.0 许可证分发
	if communityEdition {
		currentEdition = &Edition{Edition: "community", MaxServers: -1, DaysLeft: -1}
		log.Println("[license] 社区版（开源构建）运行中，无服务器数量与使用期限限制")
		return
	}
	var lic model.License
	err := dao.DB.Order("id ASC").First(&lic).Error

	if err == nil {
		// DB 中存在 License 记录：校验有效性
		if lic.Status == "revoked" {
			log.Println("[license] License 已被吊销，降级为个人版")
			downgradeToPersonal()
			return
		}
		// 启动时重新校验机器绑定与授权码密码学有效性，
		// 防止数据库文件被直接复制到其他未授权机器上继续使用，
		// 并防止通过直接修改 DB 字段（edition/max_servers/expire_at）绕过限制。
		info, trialExp, trialMax, err := reverifyStoredLicense(&lic)
		if err != nil {
			downgradeNotice = err.Error()
			log.Printf("[license] 授权记录在本机校验失败：%v，降级为个人版", err)
			downgradeToPersonal()
			return
		}
		now := time.Now()

		if info != nil {
			// 正式授权码：运行版本完全由解密并验签后的授权码内容决定，
			// DB 中的 edition/max_servers/expire_at 字段被篡改不产生任何影响。
			expireAt, perr := time.Parse("2006-01-02", info.ExpireAt)
			if perr != nil || now.After(expireAt) {
				log.Println("[license] License 已过期或日期无效，降级为个人版")
				lic.Status = "expired"
				dao.DB.Save(&lic)
				downgradeToPersonal()
				return
			}
			edition := info.Edition
			maxServers := info.MaxServers
			// 版本固定上限由程序强制，忽略 DB/授权码中的其他值；
			// 未知版本一律按个人版处理（防伪造/未来版本降级安全）
			switch edition {
			case "personal":
				maxServers = PersonalMaxServers
			case "sme":
				maxServers = SMEMaxServers
			case "enterprise":
				// -1 不限；如带具体数量 ≤10，则按产品分层归一为中小企业板
				if maxServers >= 0 && maxServers <= SMEMaxServers {
					edition = "sme"
					maxServers = SMEMaxServers
				}
			default:
				edition = "personal"
				maxServers = PersonalMaxServers
			}
			isTrial := false
			expireStr := expireAt.Format("2006-01-02")
			if expireAt.Year() >= 2099 {
				currentEdition = &Edition{
					Edition: edition, MaxServers: maxServers,
					ExpireAt: expireStr, IssuedTo: info.IssuedTo,
					IsTrial: isTrial, DaysLeft: -1,
				}
				log.Printf("[license] %s版运行中（License 有效），永久授权，无使用期限限制", editionName(edition))
				return
			}
			daysLeft := int(expireAt.Sub(now).Hours() / 24)
			currentEdition = &Edition{
				Edition: edition, MaxServers: maxServers,
				ExpireAt: expireStr, IssuedTo: info.IssuedTo,
				IsTrial: isTrial, DaysLeft: daysLeft,
			}
			log.Printf("[license] %s版运行中（License 有效），到期: %s，剩余 %d 天",
				editionName(edition), currentEdition.ExpireAt, currentEdition.DaysLeft)
			return
		}

		// 试用记录：过期时间与服务器上限来自 HMAC 保护的 Content，
		// 直接修改 DB 的 expire_at/max_servers 列无效。
		if now.After(trialExp) {
			log.Println("[license] 试用 License 已过期，降级为个人版")
			lic.Status = "expired"
			dao.DB.Save(&lic)
			downgradeToPersonal()
			return
		}
		isTrial := lic.IssuedTo == ""
		expireStr := trialExp.Format("2006-01-02")
		if trialExp.Year() >= 2099 {
			currentEdition = &Edition{
				Edition: lic.Edition, MaxServers: trialMax,
				ExpireAt: expireStr, IssuedTo: lic.IssuedTo,
				IsTrial: isTrial, DaysLeft: -1,
			}
			log.Printf("[license] %s版运行中（License 有效），永久授权，无使用期限限制", editionName(lic.Edition))
			return
		}
		daysLeft := int(trialExp.Sub(now).Hours() / 24)
		currentEdition = &Edition{
			Edition:    lic.Edition,
			MaxServers: trialMax,
			ExpireAt:   expireStr,
			IssuedTo:   lic.IssuedTo,
			IsTrial:    isTrial,
			DaysLeft:   daysLeft,
		}
		log.Printf("[license] %s版运行中（License 有效），到期: %s，剩余 %d 天",
			editionName(lic.Edition), currentEdition.ExpireAt, currentEdition.DaysLeft)
		return
	}

	// DB 无记录 → 按 config.yaml 的 edition 决定
	if cfgEdition == "personal" {
		currentEdition = &Edition{Edition: "personal", MaxServers: 3, DaysLeft: -1}
		log.Printf("[license] 个人版运行中，服务器限制 %d 台", currentEdition.MaxServers)
		return
	}

	// config 为 enterprise 但 DB 无 License → 创建 30 天试用
	createTrial()
}

func createTrial() {
	expireAt := time.Now().Add(30 * 24 * time.Hour)
	lic := model.License{
		Edition: "enterprise", MaxServers: -1, ExpireAt: expireAt,
		IssuedTo: "", Content: buildTrialContent(GetMachineHash(), expireAt, -1), Status: "active",
	}
	if err := dao.DB.Create(&lic).Error; err != nil {
		log.Printf("[license] 创建试用记录失败: %v，降级为个人版", err)
		downgradeToPersonal()
		return
	}
	currentEdition = &Edition{
		Edition: "enterprise", MaxServers: -1,
		ExpireAt: expireAt.Format("2006-01-02"), IsTrial: true, DaysLeft: 30,
	}
	log.Printf("[license] 企业版 30 天试用已启动，到期: %s", currentEdition.ExpireAt)
}

func downgradeToPersonal() {
	currentEdition = &Edition{Edition: "personal", MaxServers: 3, DaysLeft: -1}
}

// 试用记录 Content 字段格式：
//
//	TRIAL2:<machine_hash>|<expire_rfc3339>|<max_servers>|<hmac_sha256_hex>
//
// 字段间使用 "|" 分隔（RFC3339 时间含 ":"，不能用 ":" 分隔）。
// HMAC 使用内嵌 AES 密钥，覆盖 machine_hash + 过期时间 + 服务器上限，
// 从而阻止通过直接修改数据库（改 hash / expire_at / max_servers 列）来伪造试用授权。
const (
	trialPrefix   = "TRIAL:"  // 旧版明文格式（仅机器指纹），启动时自动升级为 TRIAL2
	trialV2Prefix = "TRIAL2:" // HMAC 绑定格式
)

// trialHmac 计算试用记录的 HMAC-SHA256
func trialHmac(payload string) string {
	mac := hmac.New(sha256.New, getLicenseKey())
	mac.Write([]byte(payload))
	return hex.EncodeToString(mac.Sum(nil))
}

func trialV2Payload(hash, expStr, maxStr string) string {
	return "TRIAL2|" + hash + "|" + expStr + "|" + maxStr
}

// buildTrialContent 构造 HMAC 保护的试用记录 Content
func buildTrialContent(hash string, exp time.Time, max int) string {
	expStr := exp.Format(time.RFC3339)
	maxStr := strconv.Itoa(max)
	mac := trialHmac(trialV2Payload(hash, expStr, maxStr))
	return trialV2Prefix + hash + "|" + expStr + "|" + maxStr + "|" + mac
}

// verifyTrial 校验试用记录，返回受 HMAC 保护的过期时间与服务器上限
func verifyTrial(lic *model.License) (time.Time, int, error) {
	content := strings.TrimSpace(lic.Content)
	machineHash := GetMachineHash()

	// v2：HMAC 绑定
	if strings.HasPrefix(content, trialV2Prefix) {
		rest := strings.TrimPrefix(content, trialV2Prefix)
		parts := strings.SplitN(rest, "|", 4)
		if len(parts) != 4 {
			return time.Time{}, 0, fmt.Errorf("试用授权格式错误")
		}
		boundHash, expStr, maxStr, mac := parts[0], parts[1], parts[2], parts[3]
		if boundHash != machineHash {
			return time.Time{}, 0, fmt.Errorf("试用授权与当前机器不匹配（授权创建于其他机器）")
		}
		if trialHmac(trialV2Payload(boundHash, expStr, maxStr)) != mac {
			return time.Time{}, 0, fmt.Errorf("试用授权校验失败，可能已被篡改")
		}
		exp, err := time.Parse(time.RFC3339, expStr)
		if err != nil {
			return time.Time{}, 0, fmt.Errorf("试用授权日期格式错误")
		}
		max, err := strconv.Atoi(maxStr)
		if err != nil {
			return time.Time{}, 0, fmt.Errorf("试用授权上限格式错误")
		}
		return exp, max, nil
	}

	// v1 旧版：TRIAL:<hash>，校验机器指纹后自动升级为 HMAC 格式
	if strings.HasPrefix(content, trialPrefix) {
		bound := strings.TrimPrefix(content, trialPrefix)
		if bound != machineHash {
			return time.Time{}, 0, fmt.Errorf("试用授权与当前机器不匹配（授权创建于其他机器）")
		}
		exp, max := lic.ExpireAt, lic.MaxServers
		lic.Content = buildTrialContent(machineHash, exp, max)
		dao.DB.Save(lic)
		log.Println("[license] 旧版试用记录已升级为 HMAC 绑定格式")
		return exp, max, nil
	}

	return time.Time{}, 0, fmt.Errorf("试用授权内容缺失或格式错误")
}

// reverifyStoredLicense 启动时对 DB 中的授权记录重新做本机绑定校验。
// 返回：
//   - info 非 nil：正式授权码，运行版本应完全由 info 派生；
//   - info 为 nil：试用记录，trialExp/trialMax 为受 HMAC 保护的过期时间与上限；
//   - err 非 nil：校验失败，应降级为个人版。
//
// 设计目标：数据库文件被复制到其他机器、或 DB 字段被直接篡改时，无法获得企业版能力。
func reverifyStoredLicense(lic *model.License) (*LicenseInfo, time.Time, int, error) {
	content := strings.TrimSpace(lic.Content)

	// 本地试用记录（IssuedTo 为空的企业版记录）
	if lic.IssuedTo == "" && lic.Edition == "enterprise" {
		exp, max, err := verifyTrial(lic)
		if err != nil {
			return nil, time.Time{}, 0, err
		}
		return nil, exp, max, nil
	}

	// 非试用企业版/个人版必须携带授权码；空内容一律拒绝（不再历史放行）
	if content == "" {
		return nil, time.Time{}, 0, fmt.Errorf("授权记录无有效授权内容")
	}

	// 正式授权码：完整密码学重验
	ciphertext, err := base64.StdEncoding.DecodeString(content)
	if err != nil {
		return nil, time.Time{}, 0, fmt.Errorf("授权码格式错误（非法 Base64）")
	}
	plain, err := decryptLicense(ciphertext)
	if err != nil {
		return nil, time.Time{}, 0, fmt.Errorf("授权码解密失败: %w", err)
	}
	var info LicenseInfo
	if err := json.Unmarshal(plain, &info); err != nil {
		return nil, time.Time{}, 0, fmt.Errorf("授权码解析失败")
	}
	if info.Product != "QinJiaGateway" {
		return nil, time.Time{}, 0, fmt.Errorf("授权码产品不匹配")
	}
	if info.Mark != MarkPhrase {
		return nil, time.Time{}, 0, fmt.Errorf("防伪声明缺失或不匹配，可能为伪造授权文件")
	}
	expireAt, err := time.Parse("2006-01-02", info.ExpireAt)
	if err != nil {
		return nil, time.Time{}, 0, fmt.Errorf("授权码日期格式错误")
	}
	if time.Now().After(expireAt) {
		return nil, time.Time{}, 0, fmt.Errorf("授权码已过期")
	}
	// 企业版/中小企业板正式授权必须绑定机器指纹；个人版允许空指纹（免费授权不绑定主机）
	if info.Edition == "enterprise" || info.Edition == "sme" {
		if info.MachineHash == "" {
			return nil, time.Time{}, 0, fmt.Errorf("%s授权码未绑定机器指纹，不可使用", editionName(info.Edition))
		}
		if info.MachineHash != GetMachineHash() {
			return nil, time.Time{}, 0, fmt.Errorf("授权码与当前机器不匹配（机器指纹不符，授权可能复制自其他机器）")
		}
	} else if info.MachineHash != "" && info.MachineHash != GetMachineHash() {
		// 个人版若携带指纹则同样校验
		return nil, time.Time{}, 0, fmt.Errorf("授权码与当前机器不匹配（机器指纹不符）")
	}
	if !verifyECDSASignature(&info) {
		return nil, time.Time{}, 0, fmt.Errorf("授权码签名验证失败，可能已被篡改或为伪造授权码")
	}
	return &info, time.Time{}, 0, nil
}

// AutoActivateFromFile 安装时自动激活：安装程序在用户选择「我已有授权文件」时
// 会将授权文件复制为数据目录下的 license.lic，此处读取并激活。
// 激活成功后删除该文件；失败则重命名为 license.lic.failed（避免每次启动重复尝试），
// 仅记录日志，不影响后续按 config.yaml 的 edition 正常启动。
// 必须在 dao.InitDB() 之后、license.Init() 之前调用。
func AutoActivateFromFile(dataDir string) {
	path := filepath.Join(dataDir, "license.lic")
	data, err := os.ReadFile(path)
	if err != nil {
		return // 无置入的授权文件，走正常初始化流程
	}
	content := strings.TrimSpace(string(data))
	if content == "" {
		os.Remove(path)
		return
	}
	if err := Activate(content); err != nil {
		log.Printf("[license] 安装时置入的授权文件激活失败: %v（已重命名为 license.lic.failed，可在平台中手动激活）", err)
		os.Rename(path, path+".failed")
		return
	}
	os.Remove(path)
	log.Printf("[license] 安装时置入的授权文件已自动激活")
}

// ActivateFromFile 读取指定授权文件并激活；成功后删除该文件。
// 供管理工具在服务停止状态下（无法访问Web接口时）完成激活，错误原样返回给调用方。
func ActivateFromFile(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("读取授权文件失败: %w", err)
	}
	content := strings.TrimSpace(string(data))
	if content == "" {
		os.Remove(path)
		return fmt.Errorf("授权文件内容为空")
	}
	if err := Activate(content); err != nil {
		return err
	}
	os.Remove(path)
	return nil
}

// CheckStartupLimit 启动时校验：个人版若服务器数量超过限制（如被直接改库绕过），报错并退出进程。
// 弹窗原因：
//  1. 服务模式 stderr 不可见，超限拒启时用户只看到服务启动失败却不知原因 → 弹窗说明；
//  2. 数据目录被复制到非授权机器（重验失败降级）→ 弹窗告知已降级/拒启及处理方式。
//
// 弹窗由独立子进程承载（notify.NotifyAsync），不阻塞启动，服务退出后弹窗仍保留。
func CheckStartupLimit() {
	if currentEdition == nil {
		return
	}
	if currentEdition.MaxServers < 0 {
		return // 企业版不限数量
	}
	var count int64
	dao.DB.Model(&model.Server{}).Count(&count)
	if int(count) > currentEdition.MaxServers {
		reason := "当前授权为个人版"
		if downgradeNotice != "" {
			reason = downgradeNotice
		}
		notify.NotifyAsync("授权校验失败", fmt.Sprintf(
			"当前为%s版，最多支持 %d 台服务器，但数据库中已有 %d 台，服务无法启动。\n\n"+
				"原因：%s\n\n"+
				"处理方式：\n"+
				"1. 在原授权机器上运行本系统；\n"+
				"2. 或用本机机器指纹重新生成企业版授权，将授权文件放置为数据目录下的 license.lic 后重启服务，系统将自动激活。",
			editionName(currentEdition.Edition), currentEdition.MaxServers, count, reason), true)
		log.Fatalf("[license] 授权校验失败：当前为%s版，最多支持 %d 台服务器，但数据库中已有 %d 台。"+
			"请精简服务器或购买企业版授权后重启。服务已停止。",
			editionName(currentEdition.Edition), currentEdition.MaxServers, count)
	}
	if downgradeNotice != "" {
		// 数据疑似复制自其他机器：虽未超限可运行，仍需明确告知已降级
		notify.NotifyAsync("授权提示", fmt.Sprintf(
			"检测到数据库中的授权记录与当前机器不匹配（数据可能复制自其他机器），已降级为个人版（最多管理 3 台服务器）。\n\n"+
				"原因：%s\n\n"+
				"如需在本机继续使用企业版，请在系统「设置 - 授权管理」中激活本机授权，或将本机授权文件放置为数据目录下的 license.lic 后重启服务，系统将自动激活。",
			downgradeNotice), false)
	}
}

func GetEdition() *Edition { return currentEdition }

func CanAddServer() (bool, string) {
	if currentEdition == nil {
		return false, "授权系统未初始化"
	}
	if currentEdition.MaxServers < 0 {
		return true, ""
	}
	var count int64
	dao.DB.Model(&model.Server{}).Count(&count)
	if int(count) >= currentEdition.MaxServers {
		return false, fmt.Sprintf("当前为%s版，最多支持 %d 台服务器（已有 %d 台）",
			editionName(currentEdition.Edition), currentEdition.MaxServers, count)
	}
	return true, ""
}

func editionName(ed string) string {
	switch ed {
	case "enterprise":
		return "企业"
	case "sme":
		return "中小企业板"
	case "community":
		return "社区"
	default:
		return "个人"
	}
}

// LicenseInfo License 数据结构
type LicenseInfo struct {
	Product     string `json:"product"`
	Edition     string `json:"edition"`
	MaxServers  int    `json:"max_servers"`
	ExpireAt    string `json:"expire_at"`
	IssuedTo    string `json:"issued_to"`
	IssuedAt    string `json:"issued_at"`
	Nonce       string `json:"nonce"`
	MachineHash string `json:"machine_hash"`
	AppName     string `json:"app_name"` // 客户自定义软件名称（非空时必须与平台名称一致）
	DocHash     string `json:"doc_hash"` // 授权书全文指纹（防篡改绑定）
	Mark        string `json:"mark"`     // 固定防伪声明（必须与内置 MarkPhrase 完全一致）
	Sign        string `json:"sign"`
}

// Activate 激活 License（输入授权码）
// 支持两种格式：
//  1. 纯 Base64 授权码
//  2. SYSAM-2.0 风格完整证书文本（自动提取 LICENSE_CODE_BEGIN/END 之间的内容）
func Activate(licenseContent string) error {
	// 社区版（开源构建）无需商业授权
	if communityEdition {
		return fmt.Errorf("社区版无需授权，遵循 Apache-2.0 许可证自由使用；商业授权（企业版 / 品牌定制）请联系软件供应商")
	}
	// 0. 保留原文（完整授权文件需做全文完整性校验），再提取授权码块
	original := licenseContent

	// 0.1 先检查防伪标记：完整激活文件（含 TECH_BEGIN）的技术块中必须携带
	//     Base64 编码的 MARK，且解码后逐字等于内置防伪声明（在解密授权码之前校验）
	if strings.Contains(original, "TECH_BEGIN") {
		if err := verifyOuterMark(original); err != nil {
			return err
		}
	}

	licenseContent = extractLicenseCode(licenseContent)

	// 1. Base64 解码
	ciphertext, err := base64.StdEncoding.DecodeString(licenseContent)
	if err != nil {
		return fmt.Errorf("授权码格式错误（非法 Base64）")
	}

	// 2. AES-GCM 解密
	plain, err := decryptLicense(ciphertext)
	if err != nil {
		return fmt.Errorf("授权码解密失败: %w", err)
	}

	// 3. JSON 解析
	var info LicenseInfo
	if err := json.Unmarshal(plain, &info); err != nil {
		return fmt.Errorf("授权码解析失败")
	}

	// 4. 验证产品名
	if info.Product != "QinJiaGateway" {
		return fmt.Errorf("授权码产品不匹配")
	}

	// 4.5 验证固定防伪声明（密文内必须携带，与内置文字逐字一致）
	if info.Mark != MarkPhrase {
		return fmt.Errorf("授权文件防伪校验失败：防伪声明缺失或不匹配，可能为伪造授权文件")
	}

	// 5. 验证有效期
	expireAt, err := time.Parse("2006-01-02", info.ExpireAt)
	if err != nil {
		return fmt.Errorf("授权码日期格式错误")
	}
	if time.Now().After(expireAt) {
		return fmt.Errorf("授权码已过期")
	}

	// 6. 验证机器指纹
	machineHash := GetMachineHash()
	if info.MachineHash != "" && info.MachineHash != machineHash {
		return fmt.Errorf("授权码与当前机器不匹配（机器指纹不符）")
	}

	// 7. 验证 ECDSA 签名
	if !verifyECDSASignature(&info) {
		return fmt.Errorf("授权码签名验证失败，可能已被篡改或为伪造授权码")
	}

	// 7.5 完整授权文件绑定校验：正文/技术块/授权码任一处被修改即失效
	if err := verifyDocBinding(original, &info); err != nil {
		return err
	}

	// 7.6 软件名称绑定：授权文件指定了客户自定义软件名称时必须与平台当前名称一致
	if info.AppName != "" && info.AppName != config.Global.App.Name {
		return fmt.Errorf("授权文件与当前软件名称不匹配（授权软件名称：%s，当前：%s）", info.AppName, config.Global.App.Name)
	}

	// 8. 保存到数据库
	var lic model.License
	result := dao.DB.Order("id ASC").First(&lic)
	if result.Error != nil {
		lic = model.License{
			Edition: info.Edition, MaxServers: info.MaxServers,
			ExpireAt: expireAt, IssuedTo: info.IssuedTo,
			Content: licenseContent, Status: "active",
		}
		if err := dao.DB.Create(&lic).Error; err != nil {
			return err
		}
	} else {
		lic.Edition = info.Edition
		lic.MaxServers = info.MaxServers
		lic.ExpireAt = expireAt
		lic.IssuedTo = info.IssuedTo
		lic.Content = licenseContent
		lic.Status = "active"
		if err := dao.DB.Save(&lic).Error; err != nil {
			return err
		}
	}

	Init("enterprise")
	log.Printf("[license] License 激活成功: %s，到期: %s", info.IssuedTo, info.ExpireAt)
	return nil
}

// decryptLicense AES-GCM 解密授权码
func decryptLicense(ciphertext []byte) ([]byte, error) {
	key := getLicenseKey()
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, fmt.Errorf("密文太短")
	}
	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
	return gcm.Open(nil, nonce, ciphertext, nil)
}

// extractLicenseCode 从完整证书文本中提取授权码
// 支持 SYSAM/中文授权书格式（LICENSE_CODE_BEGIN/END 标记行）
// 注意：正文注释中可能提及标记字样，必须整行精确匹配，不能用 Index
// 若无标记则原样返回（兼容纯 Base64 格式）
func extractLicenseCode(content string) string {
	var codeLines []string
	inBlock := false
	for _, line := range strings.Split(content, "\n") {
		t := strings.TrimSpace(line)
		if t == "LICENSE_CODE_BEGIN" {
			inBlock = true
			continue
		}
		if t == "LICENSE_CODE_END" {
			break
		}
		if inBlock && t != "" {
			codeLines = append(codeLines, t)
		}
	}
	if len(codeLines) > 0 {
		return strings.Join(codeLines, "")
	}
	return strings.TrimSpace(content)
}

// markRe 匹配技术块中的 MARK="base64" 值
var markRe = regexp.MustCompile(`(?m)MARK="([^"]*)"`)

// verifyOuterMark 解密前的防伪前置校验：技术块必须含 MARK 字段，
// Base64 解码后必须逐字等于内置 MarkPhrase
func verifyOuterMark(content string) error {
	m := markRe.FindStringSubmatch(content)
	if m == nil {
		return fmt.Errorf("授权文件防伪校验失败：缺少防伪标记，可能为伪造授权文件")
	}
	raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(m[1]))
	if err != nil || string(raw) != MarkPhrase {
		return fmt.Errorf("授权文件防伪校验失败：防伪标记无效，可能为伪造授权文件")
	}
	return nil
}

// sign2Re 匹配技术签名块中的 SIGN2="..." 值
var sign2Re = regexp.MustCompile(`SIGN2="([^"]*)"`)

// verifyDocBinding 完整授权文件防篡改校验（对含 TECH_BEGIN 标记的完整文件生效）：
//  1. 文件头 + 授权书正文 + 技术签名块（SIGN2 值除外）的 SHA256 必须等于授权码内的 doc_hash
//     → 改正文或技术块任意字符即失效
//  2. 技术块中每处 SIGN2 十六进制值必须与授权码内的签名一致
//     → 技术签名与授权码强关联，单独替换 SIGN2 即失效
//  3. 授权码本身由 AES-GCM 认证加密保护，改任意字节即解密失败
//  4. doc_hash 参与签名运算，以上任何伪造都无法通过签名验证
func verifyDocBinding(content string, info *LicenseInfo) error {
	content = strings.TrimPrefix(content, "\uFEFF")
	lines := strings.Split(content, "\n")
	for i := range lines {
		lines[i] = strings.TrimSuffix(lines[i], "\r")
	}
	iCertB, iCertE, iTechB, iTechE := -1, -1, -1, -1
	for i, line := range lines {
		switch strings.TrimSpace(line) {
		case "CERT_BEGIN":
			iCertB = i
		case "CERT_END":
			iCertE = i
		case "TECH_BEGIN":
			iTechB = i
		case "TECH_END":
			iTechE = i
		}
	}
	if iTechB < 0 || iTechE < 0 {
		return nil // 纯授权码（无技术块），跳过文档绑定校验
	}
	if iTechE <= iTechB {
		return fmt.Errorf("授权文件结构不完整")
	}
	hasCert := (iCertB >= 0) && (iCertE >= 0)
	if hasCert && iCertE <= iCertB {
		return fmt.Errorf("授权文件结构不完整")
	}

	// 1. 全文指纹校验（与 licgen 生成算法严格一致）
	tech := sign2Re.ReplaceAllString(strings.Join(lines[iTechB+1:iTechE], "\n"), `SIGN2=""`)
	var docCore string
	if hasCert {
		// 旧格式：文件头 + 授权书正文 + "\n" + 技术块[SIGN2 置空]
		header := strings.Join(lines[:iCertB], "\n")
		cert := strings.Join(lines[iCertB+1:iCertE], "\n")
		docCore = header + cert + "\n" + tech
	} else {
		// 新格式：文件头（TECH_BEGIN 之前）+ "\n" + 技术块[SIGN2 置空]
		header := strings.Join(lines[:iTechB], "\n")
		docCore = header + "\n" + tech
	}
	sum := sha256.Sum256([]byte(docCore))
	if hex.EncodeToString(sum[:]) != info.DocHash {
		return fmt.Errorf("授权文件完整性校验失败：技术签名内容已被修改")
	}

	// 2. SIGN2 与授权码内签名一致（技术签名 ↔ 授权码关联）
	raw, err := base64.StdEncoding.DecodeString(info.Sign)
	if err != nil {
		return fmt.Errorf("授权码签名数据异常")
	}
	expected := strings.ToUpper(hex.EncodeToString(raw))
	for _, m := range sign2Re.FindAllStringSubmatch(strings.Join(lines[iTechB+1:iTechE], "\n"), -1) {
		// 只保留十六进制字符，容忍格式化字符（空格/反斜杠/换行/tab）
		var sb strings.Builder
		for _, c := range strings.ToUpper(m[1]) {
			if (c >= '0' && c <= '9') || (c >= 'A' && c <= 'F') {
				sb.WriteRune(c)
			}
		}
		if sb.String() != expected {
			return fmt.Errorf("技术签名与授权码不匹配，授权文件已被修改")
		}
	}
	return nil
}

// verifyECDSASignature 验证 ECDSA-P256 签名
func verifyECDSASignature(info *LicenseInfo) bool {
	if info.Sign == "" {
		return false
	}

	signData := buildSignData(info)
	hash := sha256.Sum256([]byte(signData))

	// 解析签名（r || s，各 32 字节，P-256）
	signBytes, err := base64.StdEncoding.DecodeString(info.Sign)
	if err != nil || len(signBytes) != 64 {
		return false
	}
	r := new(big.Int).SetBytes(signBytes[:32])
	s := new(big.Int).SetBytes(signBytes[32:])

	// 加载公钥
	pubKey, err := loadPublicKey()
	if err != nil {
		log.Printf("[license] 加载公钥失败: %v", err)
		return false
	}

	return ecdsa.Verify(pubKey, hash[:], r, s)
}

func buildSignData(info *LicenseInfo) string {
	return fmt.Sprintf("product=%s&edition=%s&max_servers=%d&expire_at=%s&issued_to=%s&issued_at=%s&nonce=%s&machine_hash=%s&app_name=%s&doc_hash=%s&mark=%s",
		info.Product, info.Edition, info.MaxServers, info.ExpireAt,
		info.IssuedTo, info.IssuedAt, info.Nonce, info.MachineHash,
		info.AppName, info.DocHash, info.Mark)
}

// MarkPhrase 固定防伪声明，须与 licgen 的 MarkPhrase 完全一致
const MarkPhrase = "内蒙古勤加软件有限公司 授权（您）进行解密，我不会告您侵权！"

// loadPublicKey 加载内嵌的 ECDSA 公钥
func loadPublicKey() (*ecdsa.PublicKey, error) {
	pemData := getEmbeddedPublicKey()
	block, _ := pem.Decode([]byte(pemData))
	if block == nil {
		return nil, fmt.Errorf("无效的公钥 PEM")
	}
	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	if ecPub, ok := pub.(*ecdsa.PublicKey); ok {
		return ecPub, nil
	}
	return nil, fmt.Errorf("公钥类型不是 ECDSA")
}

// getEmbeddedPublicKey 内嵌公钥（编译时通过 -ldflags 注入，或硬编码）
// 生产环境应使用编译脚本注入，避免源码泄露公钥
func getEmbeddedPublicKey() string {
	if EmbeddedPublicKey != "" {
		return EmbeddedPublicKey
	}
	// 使用生成的常量（见 keys_generated.go）
	if _embeddedPublicKeyB64 != "" {
		data, err := base64.StdEncoding.DecodeString(_embeddedPublicKeyB64)
		if err == nil {
			return string(data)
		}
	}
	// 默认公钥（占位符，应替换为真实公钥）
	return `-----BEGIN PUBLIC KEY-----
MFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAEXAMPLE_KEY_REPLACE_WITH_REAL_KEY
EXAMPLE_KEY_REPLACE_WITH_REAL_KEY_IN_PRODUCTION
-----END PUBLIC KEY-----`
}

// EmbeddedPublicKey 编译时注入的公钥变量
var EmbeddedPublicKey string

// getLicenseKey 获取 AES 解密密钥（编译时注入或常量）
func getLicenseKey() []byte {
	if EmbeddedLicenseKey != "" {
		key, err := base64.StdEncoding.DecodeString(EmbeddedLicenseKey)
		if err == nil && len(key) == 32 {
			return key
		}
	}
	if _embeddedLicenseKeyB64 != "" {
		key, err := base64.StdEncoding.DecodeString(_embeddedLicenseKeyB64)
		if err == nil && len(key) == 32 {
			return key
		}
	}
	// 默认密钥（仅开发测试用，生产必须嵌入真实密钥）
	return []byte("QinJiaGateway-LicKey-2026-32byte!")
}

// EmbeddedLicenseKey 编译时注入的 AES 密钥（base64 编码的 32 字节）
var EmbeddedLicenseKey string

// GetMachineHash 获取机器指纹（CPU ID + MAC 地址组合哈希）
// 企业版授权强制绑定该指纹，换机或换网卡后授权失效
func GetMachineHash() string {
	var parts []string
	if runtime.GOOS == "windows" {
		if v := readWMICValue("cpu", "processorid"); v != "" {
			parts = append(parts, "CPU:"+v)
		}
	}
	// MAC 地址（Go 标准库，跨平台，取第一块非回环网卡）
	if ifaces, err := net.Interfaces(); err == nil {
		for _, iface := range ifaces {
			if iface.Flags&net.FlagLoopback != 0 || len(iface.HardwareAddr) == 0 {
				continue
			}
			parts = append(parts, "MAC:"+iface.HardwareAddr.String())
			break
		}
	}
	if len(parts) == 0 {
		if v := readWMICValue("baseboard", "serialnumber"); v != "" {
			parts = append(parts, "BOARD:"+v)
		}
	}
	if len(parts) == 0 {
		parts = append(parts, "HOST:"+os.Getenv("COMPUTERNAME"))
	}
	h := sha256.Sum256([]byte(strings.Join(parts, "|")))
	return fmt.Sprintf("%x", h[:8])
}

// GetMachineDetail 获取机器明细信息（用于授权书展示和激活时校验提示）
func GetMachineDetail() (cpuID, macAddr, hostName string) {
	cpuID = readWMICValue("cpu", "processorid")
	if ifaces, err := net.Interfaces(); err == nil {
		for _, iface := range ifaces {
			if iface.Flags&net.FlagLoopback != 0 || len(iface.HardwareAddr) == 0 {
				continue
			}
			macAddr = iface.HardwareAddr.String()
			break
		}
	}
	hostName, _ = os.Hostname()
	return
}

// readWMICValue 读取硬件属性值：
// 优先 wmic（旧版系统）；wmic 不存在或取不到时回退 PowerShell CIM。
// 背景：Windows 11 24H2 起 wmic 默认被移除，exec.LookPath 即失败，
// 导致 CPU ID 采集不到、机器指纹退化为仅 MAC。
func readWMICValue(component, property string) string {
	if v, ok := execWMIC(component, property); ok && v != "" {
		return v
	}
	return readCIMValue(component)
}

// execWMIC 通过 wmic 读取；ok=false 表示 wmic 不可用（应回退 CIM）
func execWMIC(component, property string) (string, bool) {
	cmd := exec.Command("wmic", component, "get", property, "/value")
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
