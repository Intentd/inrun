package structured

import (
	"reflect"
	"sort"
)

// MissingPaths returns, sorted, the dotted paths of want that got does not
// hold with the same value. Maps are walked; any other value, lists included,
// is compared whole after JSON normalisation. Keys only in got are ignored,
// and a nil in want is not required to be present.
func MissingPaths(want, got map[string]interface{}) []string {
	gotNorm, _ := JSONNormalize(got).(map[string]interface{})
	wantNorm, _ := JSONNormalize(want).(map[string]interface{})
	var out []string
	walkMissing("", wantNorm, gotNorm, &out)
	sort.Strings(out)
	return out
}

func walkMissing(prefix string, want, got map[string]interface{}, out *[]string) {
	for k, wv := range want {
		path := k
		if prefix != "" {
			path = prefix + "." + k
		}
		gv, ok := got[k]
		if !ok {
			if wv != nil {
				*out = append(*out, path)
			}
			continue
		}
		wm, wantMap := wv.(map[string]interface{})
		gm, gotMap := gv.(map[string]interface{})
		if wantMap && gotMap {
			walkMissing(path, wm, gm, out)
			continue
		}
		if !reflect.DeepEqual(wv, gv) {
			*out = append(*out, path)
		}
	}
}
