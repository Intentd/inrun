package note

import (
	"fmt"
	"text/template"
)

func semverNotes() template.FuncMap {
	return template.FuncMap{
		"semverMajor":      noteSemverMajor,
		"semverMinor":      noteSemverMinor,
		"semverPatch":      noteSemverPatch,
		"semverValid":      noteSemverValid,
		"semverCompare":    noteSemverCompare,
		"semverBump":       noteSemverBump,
		"semverConstraint": noteSemverConstraint,
	}
}

func noteSemverMajor(v string) string {
	n, err := semverMajor(v)
	if err != nil {
		return ""
	}
	return fmt.Sprintf("%d", n)
}

func noteSemverMinor(v string) string {
	n, err := semverMinor(v)
	if err != nil {
		return ""
	}
	return fmt.Sprintf("%d", n)
}

func noteSemverPatch(v string) string {
	n, err := semverPatch(v)
	if err != nil {
		return ""
	}
	return fmt.Sprintf("%d", n)
}

func noteSemverValid(v string) bool {
	return semverValid(v)
}

func noteSemverCompare(a, b string) int {
	n, _ := semverCompare(a, b)
	return n
}

func noteSemverBump(v, component string) string {
	result, err := semverBump(v, component)
	if err != nil {
		return v
	}
	return result
}

func noteSemverConstraint(v, constraint string) bool {
	ok, _ := semverCheck(v, constraint)
	return ok
}
