package businessapikeys

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
)

const (
	EnvLive = "live"
	EnvTest = "test"
)

// GeneratedKey 是一次性生成结果。Plaintext 只在创建时返回一次，绝不入库。
type GeneratedKey struct {
	Plaintext string
	Prefix    string
	Last4     string
}

func newAPIKeyID() string {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return fmt.Sprintf("apikey_%d", time.Now().UnixNano())
	}
	return "apikey_" + hex.EncodeToString(raw[:])
}

// GenerateKey 产出形如 poom_live_<48hex> 的明文 key。
func GenerateKey(env string) (GeneratedKey, error) {
	env = strings.ToLower(strings.TrimSpace(env))
	if env != EnvLive && env != EnvTest {
		env = EnvLive
	}
	var raw [24]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return GeneratedKey{}, err
	}
	plaintext := "poom_" + env + "_" + hex.EncodeToString(raw[:])
	return GeneratedKey{
		Plaintext: plaintext,
		Prefix:    "poom_" + env,
		Last4:     plaintext[len(plaintext)-4:],
	}, nil
}

// KeyHash 用 server secret 做 HMAC-SHA256，库内只存这个 hash。
func KeyHash(secret, plaintext string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(strings.TrimSpace(plaintext)))
	return hex.EncodeToString(mac.Sum(nil))
}
