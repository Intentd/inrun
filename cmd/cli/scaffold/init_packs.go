//go:build !runtime && !gateway

package scaffold

import (
	"path/filepath"
	"sort"

	"github.com/intentd/inrun/examples"
)

// Pack is an example directory that `inrun init --pack` extracts.
type Pack struct {
	Name        string
	Description string
	Path        string // directory inside the embedded FS
	First       string // example to start with, relative to Path; "" when Path is one example
	Order       int    // position in --list
}

var Packs = map[string]Pack{
	"declarative": {
		Name:        "declarative",
		Description: "Start here. Operators declared in a Blueprint, no code.",
		Path:        "reconcilers/declarative",
		First:       "01-hello-website",
		Order:       1,
	},
	"remote": {
		Name:        "remote",
		Description: "A reconciler in any language, called over HTTP.",
		Path:        "reconcilers/remote",
		First:       "endpoint",
		Order:       2,
	},
	"typed": {
		Name:        "typed",
		Description: "Go reconcilers: hooks, a constructor, and a controller-runtime drop-in.",
		Path:        "reconcilers/typed",
		First:       "hooks",
		Order:       3,
	},
	"intent": {
		Name:        "intent",
		Description: "Intent in through the gateway, view out.",
		Path:        "intent",
		Order:       4,
	},
}

func GetPack(name string) (Pack, bool) {
	if p, ok := Packs[name]; ok {
		return p, true
	}
	// Sub-path fallback: any valid directory in the embedded FS works as a pack.
	// inrun init my-project --pack reconcilers/typed/hooks extracts into my-project/hooks.
	if f, err := examples.FS.Open(name); err == nil {
		f.Close()
		return Pack{Name: filepath.Base(name), Path: name}, true
	}
	return Pack{}, false
}

func ListPacks() []Pack {
	out := make([]Pack, 0, len(Packs))
	for _, p := range Packs {
		out = append(out, p)
	}

	// Sort
	sort.Slice(out, func(i, j int) bool {
		return out[i].Order < out[j].Order
	})

	return out
}

func (p Pack) String() string {
	if p.First == "" {
		return p.Name + " — " + p.Description + " (examples/" + p.Path + ")"
	}
	return p.Name + " — " + p.Description + " (start with: examples/" + p.Path + "/" + p.First + ")"
}
