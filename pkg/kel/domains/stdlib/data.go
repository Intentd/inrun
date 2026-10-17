package note

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"text/template"
)

// Data transformation
// - toBase64
// - fromBase64
// - toJSON
// - sha256sum
// - truncate
// - slugify
// - repoSlug
// - lookup

func dataNotes() template.FuncMap {
	return template.FuncMap{
		"toBase64":     noteToBase64,
		"fromBase64":   noteFromBase64,
		"toJSON":       noteToJSON,
		"sha256sum":    noteSHA256Sum,
		"truncateName": noteTruncate,
		"slugify":      noteSlugify,
		"repoSlug":     noteRepoSlug,
		"lookup":       noteLookup,
	}
}

// ── Data transformation notes ─────────────────────────────────────────────────

// noteToBase64 base64-encodes a string. Used for Secret.data values.
//
//	{{ toBase64 .spec.password }}
func noteToBase64(s string) string {
	return base64.StdEncoding.EncodeToString([]byte(s))
}

// noteFromBase64 decodes a base64 string. Used to read Secret.data values
// in status fields or cross-CRD templates.
//
//	{{ fromBase64 .children.secret.data.password }}
func noteFromBase64(s string) string {
	b, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return ""
	}
	return string(b)
}

// noteToJSON marshals any value to a JSON string.
// Useful for embedding structured data in annotations or ConfigMap values.
//
//	{{ toJSON .spec.config }}
func noteToJSON(v interface{}) string {
	b, err := json.Marshal(v)
	if err != nil {
		return "{}"
	}
	return string(b)
}

// noteSHA256Sum returns the first 8 characters of the SHA256 hash of a string.
// Use for deterministic, stable derived names and change detection:
//
//	{{ sha256sum .spec.config }}             → "a3f5c2b1"
//	name: "config-{{ sha256sum .spec.config }}"  → content-addressed naming
func noteSHA256Sum(s string) string {
	h := sha256.Sum256([]byte(s))
	return fmt.Sprintf("%x", h[:4])
}

// noteTruncate hard-truncates s to maxLen characters with no suffix — for
// generating valid Kubernetes resource names where "..." is not a legal character.
// Use truncate (from strings notes) when displaying values to humans.
//
//	name: "{{ truncateName .spec.projectName 50 }}-deployment"
func noteTruncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen]
}

var (
	slugNonAlnum  = regexp.MustCompile(`[^a-z0-9-]`)
	slugMultiDash = regexp.MustCompile(`-+`)
)

// noteSlugify converts a string to a Kubernetes-safe name segment.
// Lowercases, replaces non-alphanumeric characters with dashes,
// and trims leading/trailing dashes.
//
//	{{ slugify "My App / Service" }}  → "my-app-service"
//	name: "{{ slugify .spec.teamName }}-operator"
func noteSlugify(s string) string {
	s = strings.ToLower(s)
	s = slugNonAlnum.ReplaceAllString(s, "-")
	s = slugMultiDash.ReplaceAllString(s, "-")
	return strings.Trim(s, "-")
}

// noteRepoSlug extracts a Kubernetes-safe name from a repository reference —
// the last path segment (repo name), with a trailing ".git" stripped, then
// slugified. Works with git URLs, org/repo shorthand, or a bare repo name.
//
//	{{ repoSlug "myorg/payments-api" }}                    → "payments-api"
//	{{ repoSlug "git@github.com:myorg/payments-api.git" }} → "payments-api"
//	{{ repoSlug "https://github.com/myorg/payments-api" }} → "payments-api"
//	{{ repoSlug "payments-api" }}                          → "payments-api"
func noteRepoSlug(s string) string {
	s = strings.TrimSuffix(strings.TrimRight(s, "/"), ".git")
	if i := strings.LastIndexAny(s, "/:"); i >= 0 {
		s = s[i+1:]
	}
	return noteSlugify(s)
}

// noteLookup returns the value paired with key in a flat list of alternating
// key/value pairs — the first match wins. Returns "" if key matches none,
// consistent with missingkey=zero elsewhere. A trailing unpaired argument is
// ignored rather than causing an error.
//
//	{{ lookup .spec.repository
//	     "myorg/payments-api"  "team-payments"
//	     "myorg/orders-api"    "team-orders"
//	     "myorg/inventory-api" "team-inventory" }}
func noteLookup(key string, pairs ...string) string {
	for i := 0; i+1 < len(pairs); i += 2 {
		if pairs[i] == key {
			return pairs[i+1]
		}
	}
	return ""
}
