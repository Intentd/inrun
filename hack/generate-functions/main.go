// Command generate-functions documents the template functions Inrun adds to
// KEL's. It reads the doc comments in pkg/template/funcs, the same way KEL
// documents its own, and writes:
//
//  1. pkg/template/funcs/funcs_generated.go — Inrun's functions and domains,
//     merged with KEL's by funcs.Registry for the CLI
//  2. docs/reference/inrun-functions/<domain>.md — one reference page per
//     domain, KEL's and Inrun's together
//
// Functions that replace one of KEL's under the same name (the adapters) are
// left out: KEL's documentation already describes them.
//
// Usage:
//
//	go run ./hack/generate-functions
//	make generate-functions
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/intentd/kel"
	"github.com/intentd/kel/gendoc"
)

const (
	funcsDir = "pkg/template/funcs"
	pagesDir = "docs/reference/inrun-functions"
)

// Registry is the Go file this command writes.
var Registry = gendoc.Registry{
	Package:   "funcs",
	Generator: "hack/generate-functions",
	Qualifier: "kel.",
	Import:    "github.com/intentd/kel",
	Functions: "inrunFunctions",
	Domains:   "inrunDomains",
}

func main() {
	fns, domains, err := gendoc.Parse(funcsDir)
	if err != nil {
		fatalf("doc comments in %s are incomplete:\n%v", funcsDir, err)
	}
	fns, domains = withoutOverrides(fns, domains, kel.Builtin())

	src, err := gendoc.Render(Registry, fns, domains)
	if err != nil {
		fatalf("rendering registry: %v", err)
	}
	out := filepath.Join(funcsDir, "funcs_generated.go")
	if err := os.WriteFile(out, src, 0o644); err != nil {
		fatalf("writing %s: %v", out, err)
	}
	fmt.Printf("generated %s (%d functions)\n", out, len(fns))

	reg := kel.Builtin().With(kel.Registry{Functions: toInfo(fns), Domains: toDomainInfo(domains)})
	for _, d := range reg.Domains {
		page := renderPage(d, reg)
		path := filepath.Join(pagesDir, d.Name+".md")
		if err := os.WriteFile(path, []byte(page), 0o644); err != nil {
			fatalf("writing %s: %v", path, err)
		}
	}
	fmt.Printf("generated %d pages in %s (%d functions)\n", len(reg.Domains), pagesDir, len(reg.Functions))
}

// withoutOverrides drops the functions KEL already has, and the domains left
// with none.
func withoutOverrides(fns []gendoc.Function, domains []gendoc.Domain, builtin kel.Registry) ([]gendoc.Function, []gendoc.Domain) {
	var kept []gendoc.Function
	used := map[string]bool{}
	for _, f := range fns {
		if _, ok := builtin.Get(f.Name); ok {
			continue
		}
		kept = append(kept, f)
		used[f.Domain] = true
	}
	var keptDomains []gendoc.Domain
	for _, d := range domains {
		if used[d.Name] {
			keptDomains = append(keptDomains, d)
		}
	}
	return kept, keptDomains
}

// renderPage writes one domain's reference page: the intro, a table of its
// functions, then each function in full.
func renderPage(d kel.DomainInfo, reg kel.Registry) string {
	fns := reg.ByDomain(d.Name)
	var b strings.Builder
	fmt.Fprintf(&b, "# %s\n\n", d.Title)
	if d.Intro != "" {
		b.WriteString(d.Intro)
		b.WriteString("\n\n")
	}

	b.WriteString("## Reference\n\n| Function | Description |\n|----------|-------------|\n")
	for _, f := range fns {
		fmt.Fprintf(&b, "| [`%s`](#%s) | %s |\n", f.Name, strings.ToLower(f.Name), tableCell(summary(f)))
	}

	b.WriteString("\n## Functions\n")
	for _, f := range fns {
		fmt.Fprintf(&b, "\n### %s\n\n%s\n\n```text\n%s\n```\n", f.Name, prose(codeNames(f.Description, f.Name)), f.Example)
		if len(f.SeeAlso) > 0 {
			links := make([]string, len(f.SeeAlso))
			for i, s := range f.SeeAlso {
				links[i] = link(s, f.Domain, reg)
			}
			fmt.Fprintf(&b, "\nSee also: %s\n", strings.Join(links, ", "))
		}
	}
	return b.String()
}

// link points at a function's section, on another page when it is in
// another domain. Links name the .md file so they work on GitHub; the
// website's sync-docs.sh rewrites them to its own URLs.
func link(name, domain string, reg kel.Registry) string {
	f, ok := reg.Get(name)
	if !ok || f.Domain == domain {
		return fmt.Sprintf("[`%s`](#%s)", name, strings.ToLower(name))
	}
	return fmt.Sprintf("[`%s`](%s.md#%s)", name, f.Domain, strings.ToLower(name))
}

// codeNames sets the function's name in code where the description starts
// with it.
func codeNames(desc, name string) string {
	if strings.HasPrefix(desc, name+" ") {
		return "`" + name + "`" + strings.TrimPrefix(desc, name)
	}
	return desc
}

// summary returns the first sentence of f's description without the
// leading name: "Returns the …" for "getServeAlias returns the …".
func summary(f kel.FunctionInfo) string {
	s := firstSentence(f.Description)
	if rest, ok := strings.CutPrefix(s, f.Name+" "); ok && rest != "" {
		s = strings.ToUpper(rest[:1]) + rest[1:]
	}
	return s
}

// firstSentence returns s up to the first sentence end in its first
// paragraph. Versions ("1.2.3") and abbreviations ("e.g.", "i.e.") are not
// sentence ends.
func firstSentence(s string) string {
	s, _, _ = strings.Cut(s, "\n")
	for i := 0; i+1 < len(s); i++ {
		if s[i] != '.' || s[i+1] != ' ' {
			continue
		}
		before := s[:i+1]
		if strings.HasSuffix(before, "e.g.") || strings.HasSuffix(before, "i.e.") || strings.HasSuffix(before, "vs.") {
			continue
		}
		return before
	}
	return s
}

func tableCell(s string) string {
	return strings.ReplaceAll(prose(s), "|", `\|`)
}

// prose escapes angle brackets outside code spans, so "-<ordinal>" is not
// read as HTML.
func prose(s string) string {
	parts := strings.Split(s, "`")
	for i := 0; i < len(parts); i += 2 {
		parts[i] = strings.NewReplacer("<", "&lt;", ">", "&gt;").Replace(parts[i])
	}
	return strings.Join(parts, "`")
}

func toInfo(fns []gendoc.Function) []kel.FunctionInfo {
	out := make([]kel.FunctionInfo, len(fns))
	for i, f := range fns {
		out[i] = kel.FunctionInfo{Name: f.Name, Domain: f.Domain, Description: f.Description, Example: f.Example, SeeAlso: f.SeeAlso, Keywords: f.Keywords}
	}
	return out
}

func toDomainInfo(domains []gendoc.Domain) []kel.DomainInfo {
	out := make([]kel.DomainInfo, len(domains))
	for i, d := range domains {
		out[i] = kel.DomainInfo{Name: d.Name, Title: d.Title, Intro: d.Intro}
	}
	return out
}

func fatalf(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, "generate-functions: "+format+"\n", args...)
	os.Exit(1)
}
