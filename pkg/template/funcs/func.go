package funcs

import (
	"maps"
	"text/template"

	"github.com/intentd/kel"
	"github.com/intentd/kel/core"
)

// withKEL is KEL's functions plus Inrun's, composed once.
var withKEL = compose()

// WithKEL returns KEL's functions plus Inrun's: a copy, so a caller can add
// its own without changing the shared map.
func WithKEL() template.FuncMap {
	return maps.Clone(withKEL)
}

// Registry returns the documentation of every function WithKEL provides:
// KEL's and Inrun's.
func Registry() kel.Registry {
	return kel.Builtin().With(kel.Registry{Functions: inrunFunctions, Domains: inrunDomains})
}

func compose() template.FuncMap {
	m := kel.FuncMap()
	register(m, serveFuncs())
	// Inrun's resolver carries structured values through strings, so these
	// replace KEL's plain versions.
	kel.Override(m, adapterFuncs())
	return m
}

// register merges src into dst.
func register(dst, src template.FuncMap) {
	kel.Register(dst, src)
}

// TypeOf returns the type name of any interface{} value.
func TypeOf(v interface{}) string {
	return core.TypeOf(v)
}
