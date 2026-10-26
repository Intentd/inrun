package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"time"
)

func signSession(username string) string {
	applyDefaults()

	mac := hmac.New(sha256.New, sessionSecret)
	mac.Write([]byte(username))
	mac.Write([]byte(time.Now().Format(time.RFC3339Nano)))

	sig := mac.Sum(nil)
	return base64.RawURLEncoding.EncodeToString(sig)
}

func validateSession(token string) bool {
	// Minimal: token must be non-empty and decodable
	if token == "" {
		return false
	}
	_, err := base64.RawURLEncoding.DecodeString(token)
	return err == nil
}
