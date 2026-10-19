package types

import (
	"fmt"
	"sort"
	"strings"
)

// Labels is a set of Kubernetes labels or annotations. When used in hook template
// declarations, values support Go text/template expressions evaluated against the
// live CR at reconcile time.
// e.g. {app: "{{ .metadata.name }}"}
type Labels map[string]string

// Stringifier
func (l Labels) String() string {
	if len(l) == 0 {
		return ""
	}
	parts := make([]string, 0, len(l))
	for k, v := range l {
		parts = append(parts, fmt.Sprintf("%s=%s", k, v))
	}
	sort.Strings(parts)
	return strings.Join(parts, ",")
}

func (l Labels) Contains(lbl map[string]string) bool {
	if l.Empty() {
		return false
	}
	for kl, vl := range l {
		for lblKey, lblVaal := range lbl {
			if kl == lblKey && vl == lblVaal {
				return true
			}
		}
	}
	return false
}

func (l Labels) Empty() bool {
	return len(l) == 0
}
