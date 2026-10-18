package funcs

import (
	"text/template"

	"github.com/intentd/kel/core"
)

// adapterFuncs — Adapters
//
// Domain: cron
//
// Inrun's versions of KEL functions whose result must survive Inrun's string-
// only resolver path: a structured value is returned as JSON with a sentinel
// prefix, and decoded back where it is used. They replace KEL's versions under
// the same names.
func adapterFuncs() template.FuncMap {
	return template.FuncMap{
		"cronToMap": cronToMapTemplate,
	}
}

// cronToMapTemplate converts a cron string to the structured map shape. For
// use in **conversion path specs only** — the result is a nested map, not a
// plain string.
//
// Keywords: cron, schedule, parse, fields, map, convert, structured
//
// Example:
//
//	# v1 → v2 conversion path
//	- from: v1
//	  to: v2
//	  spec:
//	    schedule: "{{ cronToMap .spec.schedule }}"
func cronToMapTemplate(v interface{}) (string, error) {
	return EncodeJSONValueSentinel(core.CronToMap(v))
}
