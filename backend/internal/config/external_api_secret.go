package config

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
)

// EnsureExternalAPISigningSecret 在对外 API 启用且签名密钥为空时，生成一个随机
// HMAC 根密钥并持久化到配置覆盖文件，返回是否生成了新密钥。已有密钥或未启用时不动。
func (c *Config) EnsureExternalAPISigningSecret() (bool, error) {
	c.mu.RLock()
	enabled := c.ExternalAPI.Enabled
	secret := strings.TrimSpace(c.ExternalAPI.SigningSecret)
	c.mu.RUnlock()

	if !enabled || secret != "" {
		return false, nil
	}

	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return false, fmt.Errorf("generate external api signing secret: %w", err)
	}
	generated := hex.EncodeToString(buf)

	if err := c.SaveOverride("external_api", "signing_secret", generated); err != nil {
		return false, fmt.Errorf("persist external api signing secret: %w", err)
	}
	return true, nil
}
