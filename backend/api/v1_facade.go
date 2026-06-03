package api

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/url"
	"strconv"
)

func toPublicStatus(internal string) string {
	switch internal {
	case "queued":
		return "queued"
	case "succeeded":
		return "succeeded"
	case "failed":
		return "failed"
	case "cancelled":
		return "cancelled"
	case "running", "cancel_requested":
		return "running"
	default:
		return "running"
	}
}

func signImageFileToken(secret, fileName string, exp int64) string {
	mac := hmac.New(sha256.New, []byte(secret))
	fmt.Fprintf(mac, "%s\n%d", fileName, exp)
	return hex.EncodeToString(mac.Sum(nil))
}

func verifyImageFileToken(secret, fileName string, exp int64, sig string) bool {
	want := signImageFileToken(secret, fileName, exp)
	return hmac.Equal([]byte(want), []byte(sig))
}

func signImageFileQuery(secret, fileName string, exp int64) string {
	sig := signImageFileToken(secret, fileName, exp)
	v := url.Values{}
	v.Set("exp", strconv.FormatInt(exp, 10))
	v.Set("sig", sig)
	return "?" + v.Encode()
}
