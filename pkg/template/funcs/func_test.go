package funcs_test

import (
	"bytes"
	"testing"
	"text/template"

	"github.com/intentd/inrun/pkg/template/funcs"
	"github.com/intentd/kel"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// WithKEL is KEL's functions plus Inrun's: every KEL name, the serve
// provenance functions, and Inrun's adapters in place of KEL's versions.
func TestWithKEL_ComposesKELAndInrun(t *testing.T) {
	m := funcs.WithKEL()
	for name := range kel.FuncMap() {
		assert.Contains(t, m, name, "KEL's %s", name)
	}
	for _, name := range []string{"getServeTarget", "getServeAlias", "hasServeTarget", "hasServeAlias", "isDirectApply"} {
		assert.Contains(t, m, name)
	}
	_, inKEL := kel.FuncMap()["getServeTarget"]
	assert.False(t, inKEL, "Inrun's functions do not leak into KEL's map")
}

// cronToMap is overridden so the map survives Inrun's string-only resolver
// path: the result carries the JSON value sentinel.
func TestWithKEL_CronToMapAdapter(t *testing.T) {
	tmpl, err := template.New("t").Funcs(funcs.WithKEL()).Parse(`{{ cronToMap "*/5 * * * *" }}`)
	require.NoError(t, err)
	var b bytes.Buffer
	require.NoError(t, tmpl.Execute(&b, nil))
	assert.True(t, funcs.HasJSONValueSentinel(b.String()))
	var v map[string]interface{}
	require.NoError(t, funcs.DecodeJSONValueSentinel(b.String(), &v))
	assert.Equal(t, "*/5", v["minute"])
}

// Each call returns a copy: adding to one does not change the next.
func TestWithKEL_ReturnsCopies(t *testing.T) {
	a := funcs.WithKEL()
	a["mine"] = func() string { return "x" }
	_, leaked := funcs.WithKEL()["mine"]
	assert.False(t, leaked)
}
