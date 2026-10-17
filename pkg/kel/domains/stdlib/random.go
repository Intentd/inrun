// pkg/note/random.go
//
// Random generation notes — for use with once: true on secrets.
//
// IMPORTANT: These notes are pure in the mathematical sense — they produce
// random output. They are NOT idempotent. They MUST only be used in template
// sources that declare once: true (SecretTemplateSource.Once), which ensures
// the template is only evaluated once (when the Secret does not yet exist).
//
// Using random notes without once: true causes a new value to be generated
// on every reconcile cycle — passwords change every 30 seconds, breaking
// every application that uses them.
//
// Correct usage:
//
//	secrets:
//	  - name: "{{ .metadata.name }}-credentials"
//	    once: true                              # ← REQUIRED with random notes
//	    data:
//	      password: "{{ randomAlphanumeric 32 }}"
//	      apiKey:   "{{ randomHex 16 }}"
//
// The notes are registered in note.Map() and available in all template
// expressions — the once: guard is the semantic safeguard, not a code restriction.
package note

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"text/template"
)

// randomNotes returns the FuncMap entries for random notes.
// Called by note.Map() to register them with the template engine.
func randomNotes() template.FuncMap {
	return template.FuncMap{
		// randomAlphanumeric n — returns n random alphanumeric characters
		"randomAlphanumeric": randomAlphanumeric,

		// randomHex n — returns 2n hex characters (n random bytes)
		"randomHex": randomHex,

		// randomBase64 n — returns URL-safe base64 from n random bytes
		"randomBase64": randomBase64,

		// uuidv4 — returns a random UUID v4 string
		"uuidv4": uuidv4,
	}
}

// charset for randomAlphanumeric
const alphanumCharset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

// randomAlphanumeric returns a cryptographically random alphanumeric string
// of exactly n characters.
//
// Template: {{ randomAlphanumeric 32 }}
// Produces: "k7Xm3pQs9vR2nTwY8cL1jF6bH0dE4gA5"
//
// Use with once: true on secrets to generate stable passwords.
func randomAlphanumeric(n int) (string, error) {
	if n <= 0 {
		return "", nil
	}
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("randomAlphanumeric: %w", err)
	}
	result := make([]byte, n)
	charsetLen := byte(len(alphanumCharset))
	for i := range result {
		result[i] = alphanumCharset[b[i]%charsetLen]
	}
	return string(result), nil
}

// randomHex returns a cryptographically random hex-encoded string.
// n is the number of random bytes — the output string is 2n characters.
//
// Template: {{ randomHex 16 }}
// Produces: "a3f8b2c1d4e5f6a7b8c9d0e1f2a3b4c5"  (32 hex chars from 16 bytes)
//
// Useful for: API keys, session tokens, CSRF tokens.
func randomHex(n int) (string, error) {
	if n <= 0 {
		return "", nil
	}
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("randomHex: %w", err)
	}
	return hex.EncodeToString(b), nil
}

// uuidv4 returns a random UUID v4 string in standard 8-4-4-4-12 hex format.
//
// Template: {{ uuidv4 }}
// Produces: "f47ac10b-58cc-4372-a567-0e02b2c3d479"
//
// Use with once: true on secrets. Same entropy as randomHex 16, formatted
// as a UUID for systems that expect that shape (Kubernetes UIDs, OAuth client
// IDs, correlation IDs, Gateway API tokens).
func uuidv4() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("uuidv4: %w", err)
	}
	b[6] = (b[6] & 0x0f) | 0x40 // version 4
	b[8] = (b[8] & 0x3f) | 0x80 // variant bits
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:16]), nil
}

// randomBase64 returns a cryptographically random URL-safe base64 string.
// n is the number of random bytes — the output length is ceil(n*4/3).
//
// Template: {{ randomBase64 32 }}
// Produces: "k7Xm3pQs9vR2nTwY8cL1jF6bH0dE4gA5zP..."
//
// Useful for: JWT secrets, HMAC keys, cookie secrets.
// URL-safe encoding (uses - and _ instead of + and /) — safe in HTTP headers.
func randomBase64(n int) (string, error) {
	if n <= 0 {
		return "", nil
	}
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("randomBase64: %w", err)
	}
	return base64.URLEncoding.EncodeToString(b), nil
}
