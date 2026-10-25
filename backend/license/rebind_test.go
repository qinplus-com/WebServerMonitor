package license

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"qinjiagateway/dao"
	"qinjiagateway/model"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func setupTestDB(t *testing.T) {
	dir := t.TempDir()
	db, err := gorm.Open(sqlite.Open(filepath.Join(dir, "test.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		sqlDB, _ := db.DB()
		sqlDB.Close()
	})
	db.AutoMigrate(&model.License{})
	dao.DB = db
}

func codeFromLic(t *testing.T, path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return extractLicenseCode(string(data))
}

func future() time.Time { return time.Now().Add(24 * time.Hour) }

// 试用 v2 HMAC 绑定：本机通过
func TestTrialV2Local(t *testing.T) {
	setupTestDB(t)
	local := GetMachineHash()
	exp := time.Now().Add(15 * 24 * time.Hour)
	lic := &model.License{
		Edition: "enterprise", IssuedTo: "",
		ExpireAt: exp, MaxServers: -1,
		Content: buildTrialContent(local, exp, -1),
	}
	info, gotExp, gotMax, err := reverifyStoredLicense(lic)
	if err != nil || info != nil || gotMax != -1 {
		t.Fatalf("本机试用应通过，got info=%v exp=%v max=%d err=%v", info != nil, gotExp, gotMax, err)
	}
}

// 试用 v2：绑定其他机器 -> 拒绝
func TestTrialV2Foreign(t *testing.T) {
	setupTestDB(t)
	exp := future()
	lic := &model.License{
		Edition: "enterprise", IssuedTo: "", ExpireAt: exp, MaxServers: -1,
		Content: buildTrialContent("0123456789abcdef", exp, -1),
	}
	_, _, _, err := reverifyStoredLicense(lic)
	if err == nil || !strings.Contains(err.Error(), "不匹配") {
		t.Fatalf("外机试用应拒绝，got: %v", err)
	}
}

// 试用 v2：攻击者改 DB max_servers 列为 999，但 Content 内 HMAC 仍是 -1
// -> 运行上限应取 Content 内的 -1，DB 列篡改无效（这里只校验校验通过，运行期由 Init 取 trialMax）
func TestTrialV2DBMaxSpoofIgnored(t *testing.T) {
	setupTestDB(t)
	local := GetMachineHash()
	exp := future()
	lic := &model.License{
		Edition: "enterprise", IssuedTo: "", ExpireAt: exp,
		MaxServers: 999, // 攻击者篡改
		Content:    buildTrialContent(local, exp, -1),
	}
	_, _, gotMax, err := reverifyStoredLicense(lic)
	if err != nil || gotMax != -1 {
		t.Fatalf("DB max_servers 篡改应被忽略，got max=%d err=%v", gotMax, err)
	}
}

// 试用 v2：攻击者直接改 Content 里的过期为永久但未重算 HMAC -> 拒绝
func TestTrialV2TamperRejected(t *testing.T) {
	setupTestDB(t)
	local := GetMachineHash()
	good := buildTrialContent(local, future(), -1)
	// 把过期段替换为 2099 年
	parts := strings.SplitN(good, "|", 4)
	parts[1] = "2099-12-31T00:00:00Z"
	tampered := strings.Join(parts, "|")
	lic := &model.License{
		Edition: "enterprise", IssuedTo: "", ExpireAt: future(), MaxServers: -1,
		Content: tampered,
	}
	_, _, _, err := reverifyStoredLicense(lic)
	if err == nil {
		t.Fatal("篡改试用过期应被 HMAC 拒绝")
	}
}

// 正式授权：本机通过，返回 info
func TestFormalLocal(t *testing.T) {
	setupTestDB(t)
	code := codeFromLic(t, os.Getenv("TEMP")+`\local.lic`)
	lic := &model.License{
		Edition: "enterprise", IssuedTo: "本机企业", ExpireAt: future(), MaxServers: -1,
		Content: code,
	}
	info, _, _, err := reverifyStoredLicense(lic)
	if err != nil || info == nil || info.Edition != "enterprise" {
		t.Fatalf("本机正式授权应返回 enterprise info，got info=%v err=%v", info, err)
	}
}

// 正式授权：绑定其他机器 -> 拒绝
func TestFormalForeign(t *testing.T) {
	setupTestDB(t)
	code := codeFromLic(t, os.Getenv("TEMP")+`\foreign.lic`)
	lic := &model.License{
		Edition: "enterprise", IssuedTo: "其他", ExpireAt: future(), MaxServers: -1,
		Content: code,
	}
	_, _, _, err := reverifyStoredLicense(lic)
	if err == nil || !strings.Contains(err.Error(), "机器指纹不符") {
		t.Fatalf("外机正式授权应拒绝，got: %v", err)
	}
}

// 核心绕过场景：DB Edition 改为 enterprise，但 Content 是个人码
// -> reverify 返回 info.Edition=personal，Init 必须据此运行个人版
func TestDBEditionSpoofIgnored(t *testing.T) {
	setupTestDB(t)
	personalCode := codeFromLic(t, os.Getenv("TEMP")+`\personal.lic`)
	lic := &model.License{
		Edition: "enterprise", IssuedTo: "伪装企业", ExpireAt: future(), MaxServers: -1,
		Content: personalCode,
	}
	info, _, _, err := reverifyStoredLicense(lic)
	if err != nil {
		t.Fatalf("个人码应通过校验，got err: %v", err)
	}
	if info.Edition != "personal" {
		t.Fatalf("应按授权码内容判定为 personal，got: %s", info.Edition)
	}
}

// 企业行 + 非空 IssuedTo + 空 Content -> 必须拒绝（旧放行漏洞）
func TestEnterpriseEmptyContentRejected(t *testing.T) {
	setupTestDB(t)
	lic := &model.License{
		Edition: "enterprise", IssuedTo: "某人", ExpireAt: future(), MaxServers: -1,
		Content: "",
	}
	_, _, _, err := reverifyStoredLicense(lic)
	if err == nil {
		t.Fatal("空 Content 企业行应拒绝")
	}
}

// 整体 Init：外机拷库（企业正式授权）-> 降级个人版
func TestInitDowngradeOnForeignMachine(t *testing.T) {
	setupTestDB(t)
	foreignCode := codeFromLic(t, os.Getenv("TEMP")+`\foreign.lic`)
	dao.DB.Create(&model.License{
		Edition: "enterprise", MaxServers: -1, ExpireAt: future(),
		IssuedTo: "其他机器企业", Content: foreignCode, Status: "active",
	})
	Init("enterprise")
	ed := GetEdition()
	if ed.Edition != "personal" {
		t.Fatalf("外机拷库启动应降级个人版，got: %s", ed.Edition)
	}
}

// 整体 Init：本机正式授权 -> 企业版，且忽略 DB 被篡改为 personal/max=3
func TestInitLocalIgnoresDBFields(t *testing.T) {
	setupTestDB(t)
	localCode := codeFromLic(t, os.Getenv("TEMP")+`\local.lic`)
	dao.DB.Create(&model.License{
		Edition: "personal", MaxServers: 3, ExpireAt: future(), // 攻击者篡改
		IssuedTo: "本机企业", Content: localCode, Status: "active",
	})
	Init("enterprise")
	ed := GetEdition()
	if ed.Edition != "enterprise" || ed.MaxServers != -1 {
		t.Fatalf("应按授权码运行企业版不限台，got edition=%s max=%d", ed.Edition, ed.MaxServers)
	}
}
