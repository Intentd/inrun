package note

import (
	"encoding/json"
	"net/mail"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"text/template"
)

// General-purpose input-format checks, exposed as notes.
//
// An IDP form field is free text until something checks it — these are the
// checks a catalog author reaches for most often: is this actually an
// email, a git repository, a URL, a container image reference, well-formed
// JSON, a valid port number. Catching a malformed value at admission time,
// with a message built from the field's label:, beats the developer
// discovering it three steps downstream when ArgoCD or cert-manager fails
// to reconcile.
//
// Usage examples:
//   {{ isValidEmail .spec.ownerEmail }}
//   {{ isValidGitRepository .spec.repoURL }}
//   {{ isValidURL .spec.webhookUrl }}
//   {{ isValidImageRef .spec.image }}
//   {{ isValidJSON .spec.serviceSelector }}
//   {{ isValidPort .spec.port }}

func validationNotes() template.FuncMap {
	return template.FuncMap{
		"isValidEmail":         noteIsValidEmail,
		"isValidGitRepository": noteIsValidGitRepository,
		"isValidURL":           noteIsValidURL,
		"isValidImageRef":      noteIsValidImageRef,
		"isValidJSON":          noteIsValidJSON,
		"isValidPort":          noteIsValidPort,
	}
}

// noteIsValidEmail reports whether s is a single, bare email address —
// "user@example.com", not a display-name form like "User <user@example.com>".
//
//	{{ isValidEmail "team@myorg.io" }}  → true
//	{{ isValidEmail "not-an-email"  }}  → false
func noteIsValidEmail(s string) bool {
	addr, err := mail.ParseAddress(s)
	if err != nil {
		return false
	}
	// ParseAddress accepts "Display Name <addr>" too — reject anything
	// that isn't just the bare address the caller passed in.
	return addr.Address == strings.TrimSpace(s)
}

// gitRepoPatterns matches the URL and SCP-like ("user@host:path") shapes a
// git remote commonly takes.
var gitRepoPatterns = []*regexp.Regexp{
	regexp.MustCompile(`^(https?|git|ssh)://[^\s]+$`),
	regexp.MustCompile(`^[\w.-]+@[\w.-]+:[\w./~-]+(\.git)?$`),
}

// noteIsValidGitRepository reports whether s looks like a git repository
// URL — https://, git://, ssh://, or the SCP-like "git@host:org/repo.git"
// shorthand. This is a shape check, not a reachability check — it doesn't
// dial the host.
//
//	{{ isValidGitRepository "https://github.com/myorg/payments"    }}  → true
//	{{ isValidGitRepository "git@github.com:myorg/payments.git"    }}  → true
//	{{ isValidGitRepository "not a repo"                            }}  → false
func noteIsValidGitRepository(s string) bool {
	if s == "" {
		return false
	}
	for _, re := range gitRepoPatterns {
		if re.MatchString(s) {
			return true
		}
	}
	return false
}

// noteIsValidURL reports whether s is an absolute http or https URL with a
// non-empty host.
//
//	{{ isValidURL "https://example.com/webhook" }}  → true
//	{{ isValidURL "ftp://example.com"            }}  → false
//	{{ isValidURL "not a url"                     }}  → false
func noteIsValidURL(s string) bool {
	u, err := url.ParseRequestURI(s)
	if err != nil {
		return false
	}
	return (u.Scheme == "http" || u.Scheme == "https") && u.Host != ""
}

// imageRefPattern is a pragmatic container image reference check —
// [registry[:port]/]repo/path[:tag|@digest] — not a full implementation of
// the OCI distribution spec, but enough to catch the common mistakes: a
// stray space, an uppercase letter, a missing tag separator.
var imageRefPattern = regexp.MustCompile(
	`^(?:[a-z0-9]+(?:[.-][a-z0-9]+)*(?::[0-9]+)?/)?` +
		`[a-z0-9]+(?:[._-]+[a-z0-9]+)*(?:/[a-z0-9]+(?:[._-]+[a-z0-9]+)*)*` +
		`(?::[\w][\w.-]{0,127}|@[A-Za-z][A-Za-z0-9]*(?:[-_+.][A-Za-z][A-Za-z0-9]*)*:[0-9A-Fa-f]{32,})?$`,
)

// noteIsValidImageRef reports whether s looks like a valid container image
// reference: optional registry, lowercase repo path, optional :tag or
// @digest.
//
//	{{ isValidImageRef "myorg/app:v1.2.3"                    }}  → true
//	{{ isValidImageRef "registry.myorg.io:5000/team/app:latest" }}  → true
//	{{ isValidImageRef "MyOrg/App"                            }}  → false
func noteIsValidImageRef(s string) bool {
	if s == "" {
		return false
	}
	return imageRefPattern.MatchString(s)
}

// noteIsValidJSON reports whether s is syntactically valid JSON.
//
//	{{ isValidJSON "{\"app\": \"payments-api\"}" }}  → true
//	{{ isValidJSON "{not json"                    }}  → false
func noteIsValidJSON(s string) bool {
	return json.Valid([]byte(s))
}

// noteIsValidPort reports whether s parses as an integer in the valid TCP/UDP
// port range, 1–65535.
//
//	{{ isValidPort "8080"  }}  → true
//	{{ isValidPort "0"     }}  → false
//	{{ isValidPort "70000" }}  → false
func noteIsValidPort(s string) bool {
	n, err := strconv.Atoi(s)
	if err != nil {
		return false
	}
	return n >= 1 && n <= 65535
}
