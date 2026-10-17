// hack/generate-notes/main.go
//
// Generates two outputs from the markdown doc files in pkg/note/docs/:
//
//  1. pkg/note/list_generated.go — note registry for the Inrun runtime and CLI
//  2. docs/reference/inrun-notes/<domain>.md — user-facing reference pages
//
// pkg/note/docs/ is the single source of truth. Run `make generate-notes` after
// adding or updating a doc file — both outputs are refreshed automatically.
//
// Files containing an "## In Development" heading are excluded from both outputs
// until the feature is ready.
//
// Usage:
//
//	go run ./hack/generate-notes
//	make generate-notes
//
// The generator parses every *.md file in docs/ and extracts note entries
// from ### `noteName` headings. The first non-empty paragraph after the
// heading becomes the description. The first fenced code block becomes the
// example. Some headings document two notes together using the form:
//
//	### `toLower` / `toUpper`
//
// In that case both names are registered with the same description and example.
//
// The domain is derived from the filename: "01-strings.md" → "strings".
package main

import (
	"bufio"
	"bytes"
	"fmt"
	"go/format"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"unicode"
)

type noteEntry struct {
	Name        string
	Domain      string
	Description string
	Example     string
	Keywords    []string
}

type domainInfo struct {
	title   string
	intro   string
	entries []noteEntry
}

var (
	headingRe  = regexp.MustCompile("^###\\s+(.+)$")
	backtickRe = regexp.MustCompile("`([^`]+)`")
)

func main() {
	docsDir := "pkg/note/docs"
	outFile := "pkg/note/list_generated.go"
	userDocsDir := "docs/reference/inrun-notes"

	files, err := filepath.Glob(filepath.Join(docsDir, "*.md"))
	if err != nil || len(files) == 0 {
		fatalf("no markdown files found in %s: %v", docsDir, err)
	}
	sort.Strings(files)

	var entries []noteEntry
	domainDocs := map[string]*domainInfo{}

	for _, f := range files {
		domain := domainFromFilename(filepath.Base(f))
		if domain == "" || domain == "readme" || domain == "_template" {
			continue
		}
		if isInDevelopment(f) {
			fmt.Printf("skipping %s (## In Development)\n", filepath.Base(f))
			continue
		}
		parsed, err := parseDoc(f, domain)
		if err != nil {
			fatalf("parsing %s: %v", f, err)
		}
		title, intro := readDomainMeta(f)
		domainDocs[domain] = &domainInfo{
			title:   title,
			intro:   intro,
			entries: parsed,
		}
		entries = append(entries, parsed...)
	}

	// Sort by domain, then name for stable output
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Domain != entries[j].Domain {
			return entries[i].Domain < entries[j].Domain
		}
		return entries[i].Name < entries[j].Name
	})

	// Write the note list
	src := renderGoFile(entries)
	formatted, err := format.Source([]byte(src))
	if err != nil {
		_ = os.WriteFile(outFile, []byte(src), 0644)
		fatalf("formatting generated source: %v\n\nsource written to %s for inspection", err, outFile)
	}
	if err := os.WriteFile(outFile, formatted, 0644); err != nil {
		fatalf("writing %s: %v", outFile, err)
	}
	fmt.Printf("generated %s (%d notes)\n", outFile, len(entries))

	// Write user-facing docs
	domains := make([]string, 0, len(domainDocs))
	for d := range domainDocs {
		domains = append(domains, d)
	}
	sort.Strings(domains)

	for _, domain := range domains {
		info := domainDocs[domain]
		page := renderUserFacingPage(info.title, info.intro, info.entries)
		outPath := filepath.Join(userDocsDir, domain+".md")
		if err := os.WriteFile(outPath, []byte(page), 0644); err != nil {
			fatalf("writing %s: %v", outPath, err)
		}
		fmt.Printf("generated %s (%d notes)\n", outPath, len(info.entries))
	}
}

// domainFromFilename converts "01-strings.md" → "strings".
func domainFromFilename(name string) string {
	name = strings.TrimSuffix(name, ".md")
	name = strings.ToLower(name)
	if name == "readme" {
		return ""
	}
	// Strip leading "NN-" prefix
	if idx := strings.Index(name, "-"); idx >= 0 && idx <= 2 {
		name = name[idx+1:]
	}
	// Normalise separators: "safe-access" → "safeaccess"
	name = strings.ReplaceAll(name, "-", "")
	return name
}

// isInDevelopment reports whether the file contains an "## In Development" heading.
func isInDevelopment(path string) bool {
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	for _, line := range strings.Split(string(data), "\n") {
		if strings.TrimSpace(line) == "## In Development" {
			return true
		}
	}
	return false
}

// readDomainMeta extracts the domain title and intro text from the doc file.
// The title is taken from "# NN — Domain Title" (words after the em-dash).
// The intro is all content between the title line and the first ## section.
func readDomainMeta(path string) (title, intro string) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", ""
	}
	lines := strings.Split(string(data), "\n")
	pastTitle := false
	var introLines []string
	for _, line := range lines {
		if strings.HasPrefix(line, "# ") && !pastTitle {
			h := strings.TrimPrefix(line, "# ")
			if idx := strings.Index(h, "—"); idx >= 0 {
				title = strings.TrimSpace(h[idx+len("—"):])
			} else {
				// Strip leading digits and spaces: "01 Strings" → "Strings"
				title = strings.TrimLeftFunc(strings.TrimSpace(h), func(r rune) bool {
					return unicode.IsDigit(r) || r == ' '
				})
			}
			pastTitle = true
			continue
		}
		if !pastTitle {
			continue
		}
		// Stop at first ## section
		if strings.HasPrefix(line, "## ") {
			break
		}
		introLines = append(introLines, line)
	}
	// Trim leading/trailing blank lines
	for len(introLines) > 0 && strings.TrimSpace(introLines[0]) == "" {
		introLines = introLines[1:]
	}
	for len(introLines) > 0 && strings.TrimSpace(introLines[len(introLines)-1]) == "" {
		introLines = introLines[:len(introLines)-1]
	}
	intro = strings.Join(introLines, "\n")
	if title == "" {
		d := domainFromFilename(filepath.Base(path))
		if len(d) > 0 {
			title = strings.ToUpper(d[:1]) + d[1:]
		}
	}
	return title, intro
}

// renderUserFacingPage generates a user-facing markdown reference page.
func renderUserFacingPage(title, intro string, entries []noteEntry) string {
	var b strings.Builder

	fmt.Fprintf(&b, "# %s\n\n", title)

	if intro != "" {
		b.WriteString(intro)
		b.WriteString("\n\n")
	}

	// Reference table
	b.WriteString("## Reference\n\n")
	b.WriteString("| Note | Description |\n")
	b.WriteString("|------|-------------|\n")
	for _, e := range entries {
		desc := firstSentence(e.Description)
		fmt.Fprintf(&b, "| `%s` | %s |\n", e.Name, desc)
	}
	b.WriteString("\n")

	// Examples — one combined yaml block with per-note comments
	b.WriteString("## Examples\n\n")
	b.WriteString("```yaml\n")
	first := true
	for _, e := range entries {
		if e.Example == "" {
			continue
		}
		if !first {
			b.WriteString("\n")
		}
		fmt.Fprintf(&b, "# %s\n", e.Name)
		b.WriteString(e.Example)
		b.WriteString("\n")
		first = false
	}
	b.WriteString("```\n")

	return b.String()
}

// firstSentence returns the first sentence of s (up to and including the first ".").
func firstSentence(s string) string {
	if idx := strings.IndexByte(s, '.'); idx >= 0 {
		return s[:idx+1]
	}
	return s
}

// parseDoc parses one markdown file and returns a slice of note entries.
func parseDoc(path, domain string) ([]noteEntry, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	type state int
	const (
		stateScanning  state = iota // looking for a ### heading
		stateDesc                   // collecting description lines
		stateCodeBlock              // inside a ``` block
		stateDone                   // have description and example for current set
	)

	var (
		entries     []noteEntry
		names       []string // current heading's note names
		descLines   []string
		exLines     []string
		keywords    []string
		inCodeBlock bool
		st          state = stateScanning
	)

	flush := func() {
		if len(names) == 0 {
			return
		}
		desc := strings.TrimSpace(strings.Join(descLines, " "))
		ex := strings.TrimSpace(strings.Join(exLines, "\n"))
		kws := make([]string, len(keywords))
		copy(kws, keywords)
		for _, name := range names {
			entries = append(entries, noteEntry{
				Name:        name,
				Domain:      domain,
				Description: desc,
				Example:     ex,
				Keywords:    kws,
			})
		}
		names = nil
		descLines = nil
		exLines = nil
		keywords = nil
		inCodeBlock = false
		st = stateScanning
	}

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()

		// Detect ### heading
		if m := headingRe.FindStringSubmatch(line); m != nil {
			extracted := extractNoteNames(m[1])
			if len(extracted) > 0 {
				flush() // save previous entry
				names = extracted
				st = stateDesc
				continue
			}
		}

		if st == stateScanning {
			continue
		}

		// Track fenced code blocks
		if strings.HasPrefix(line, "```") {
			inCodeBlock = !inCodeBlock
			if !inCodeBlock {
				// closing fence — we have the example
				st = stateDone
			}
			continue
		}

		if inCodeBlock {
			exLines = append(exLines, line)
			continue
		}

		if st == stateDesc {
			// A new ### heading (not a note) or a ## section resets state
			if strings.HasPrefix(line, "## ") || strings.HasPrefix(line, "# ") {
				flush()
				continue
			}
			// Blank line after we already have text means description is done
			trimmed := strings.TrimSpace(line)
			if trimmed == "" {
				if len(descLines) > 0 {
					// description collected; wait for optional example
				}
				continue
			}
			// Skip lines that are just separators
			if trimmed == "---" {
				flush()
				continue
			}
			// Detect "Keywords: a, b, c" line — strip from description, store separately.
			if strings.HasPrefix(strings.ToLower(trimmed), "keywords:") {
				raw := trimmed[len("keywords:"):]
				for _, k := range strings.Split(raw, ",") {
					k = strings.ToLower(strings.TrimSpace(k))
					if k != "" {
						keywords = append(keywords, k)
					}
				}
				continue
			}
			descLines = append(descLines, trimmed)
		}
	}
	flush()

	return entries, scanner.Err()
}

// extractNoteNames extracts all backtick-quoted identifiers from a heading
// fragment, filtering out non-function names (those containing spaces, dots, etc.).
func extractNoteNames(heading string) []string {
	matches := backtickRe.FindAllStringSubmatch(heading, -1)
	var names []string
	for _, m := range matches {
		name := m[1]
		// Skip if it looks like a template expression or contains special chars
		if strings.ContainsAny(name, " .{}()/\\") {
			continue
		}
		// Skip if it starts with uppercase (type, not function)
		if len(name) > 0 && name[0] >= 'A' && name[0] <= 'Z' {
			continue
		}
		names = append(names, name)
	}
	return names
}

func renderGoFile(entries []noteEntry) string {
	var b bytes.Buffer
	b.WriteString("// Code generated by hack/generate-notes. DO NOT EDIT.\n\n")
	b.WriteString("package note\n\n")
	b.WriteString("// BuiltinNotes is the complete registry of documented Inrun note functions.\n")
	b.WriteString("// Generated from pkg/note/docs/ — run `make generate-notes` to refresh.\n")
	b.WriteString("var BuiltinNotes = []NoteInfo{\n")
	for _, e := range entries {
		fmt.Fprintf(&b, "\t{\n")
		fmt.Fprintf(&b, "\t\tName:        %q,\n", e.Name)
		fmt.Fprintf(&b, "\t\tDomain:      %q,\n", e.Domain)
		fmt.Fprintf(&b, "\t\tDescription: %q,\n", e.Description)
		if e.Example != "" {
			fmt.Fprintf(&b, "\t\tExample:     %q,\n", e.Example)
		}
		if len(e.Keywords) > 0 {
			kwParts := make([]string, len(e.Keywords))
			for i, k := range e.Keywords {
				kwParts[i] = fmt.Sprintf("%q", k)
			}
			fmt.Fprintf(&b, "\t\tKeywords:    []string{%s},\n", strings.Join(kwParts, ", "))
		}
		fmt.Fprintf(&b, "\t},\n")
	}
	b.WriteString("}\n")
	return b.String()
}

func fatalf(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, "generate-notes: "+format+"\n", args...)
	os.Exit(1)
}
