package utils

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"unicode/utf8"

	"qinjiagateway/config"
)

// encPrefix 密文前缀，用于区分明文与密文（兼容迁移前的旧数据）
const encPrefix = "ENC1:"

var (
	keyOnce   sync.Once
	keyCached []byte
)

// aesKey 获取 AES-256 密钥。
// 密钥持久化在数据目录 .aeskey 文件中（随数据走，与安装目录/JWT secret 解耦），
// 保证重装程序后旧密文仍可解密；首次启动无该文件时按旧版算法从 JWT secret
// 派生并写入，已存在的 ENC1 密文因此保持可解。
func aesKey() []byte {
	keyOnce.Do(func() {
		keyCached = loadOrCreateAESKey()
	})
	return keyCached
}

// InitCrypto 启动时主动初始化密钥（在数据库迁移前调用），
// 确保密钥尽早持久化到数据目录，而非等到首次加解密时才生成
func InitCrypto() {
	aesKey()
}

func loadOrCreateAESKey() []byte {
	keyFile := ""
	if config.Global != nil && config.Global.DataDir != "" {
		keyFile = filepath.Join(config.Global.DataDir, ".aeskey")
	}
	if keyFile != "" {
		if data, err := os.ReadFile(keyFile); err == nil {
			if b, err := hex.DecodeString(strings.TrimSpace(string(data))); err == nil && len(b) == 32 {
				return b
			}
			log.Printf("[crypto] 密钥文件 %s 内容无效，按派生密钥重建", keyFile)
		}
	}
	// 派生密钥（与旧版本完全相同的算法，兼容存量密文）
	h := sha256.Sum256([]byte("qinjia-aes-key-" + config.Global.JWT.Secret))
	key := h[:]
	if keyFile != "" {
		if err := os.MkdirAll(filepath.Dir(keyFile), 0755); err == nil {
			if err := os.WriteFile(keyFile, []byte(hex.EncodeToString(key)), 0600); err != nil {
				log.Printf("[crypto] 密钥文件写入失败 %s: %v（重装后历史密文可能无法解密）", keyFile, err)
			}
		}
	}
	return key
}

// IsEncrypted 判断字符串是否为 ENC1 密文
func IsEncrypted(s string) bool {
	return strings.HasPrefix(s, encPrefix)
}

// Encrypt AES-256-CBC 加密，返回 base64(ENC1:IV+ciphertext)；空字符串原样返回
func Encrypt(plaintext string) string {
	if plaintext == "" {
		return ""
	}
	key := aesKey()
	block, err := aes.NewCipher(key)
	if err != nil {
		return plaintext
	}
	// PKCS7 填充
	pad := aes.BlockSize - len(plaintext)%aes.BlockSize
	data := append([]byte(plaintext), bytes.Repeat([]byte{byte(pad)}, pad)...)

	iv := make([]byte, aes.BlockSize)
	if _, err := io.ReadFull(rand.Reader, iv); err != nil {
		return plaintext
	}
	mode := cipher.NewCBCEncrypter(block, iv)
	mode.CryptBlocks(data, data)

	raw := append(iv, data...)
	return encPrefix + base64.StdEncoding.EncodeToString(raw)
}

// DecryptEx 解密并显式返回是否成功。
// 非密文（无前缀）按明文返回 ok=true（向后兼容）；
// ENC1 密文解密失败（密钥变更/数据损坏）返回 ok=false，调用方不得再使用原密文。
func DecryptEx(s string) (string, bool) {
	if !IsEncrypted(s) {
		return s, true
	}
	key := aesKey()
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", false
	}
	raw, err := base64.StdEncoding.DecodeString(s[len(encPrefix):])
	if err != nil || len(raw) < aes.BlockSize {
		return "", false
	}
	iv := raw[:aes.BlockSize]
	ct := raw[aes.BlockSize:]
	if len(ct)%aes.BlockSize != 0 {
		return "", false
	}
	mode := cipher.NewCBCDecrypter(block, iv)
	mode.CryptBlocks(ct, ct)
	// 去 PKCS7 填充
	pad := int(ct[len(ct)-1])
	if pad <= 0 || pad > aes.BlockSize || pad > len(ct) {
		return "", false
	}
	plain := ct[:len(ct)-pad]
	// 密钥错误有小概率凑出合法 padding；UTF-8 校验拦截此类误判
	if !utf8.Valid(plain) {
		return "", false
	}
	return string(plain), true
}

// Decrypt 解密；非密文（无前缀）按明文返回，保证向后兼容。
// 解密失败返回原字符串（旧行为）；需要区分成败的调用方应使用 DecryptEx。
func Decrypt(s string) string {
	if plain, ok := DecryptEx(s); ok {
		return plain
	}
	return s
}
