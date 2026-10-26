// hack/generate-resource-docs/main.go
//
// Generates docs/reference/schema/resources/<kind>.md from the
// *TemplateSource structs in pkg/types/types_<kind>.go — the user-facing
// schema for onCreate/onReconcile/onDelete resource declarations.
//
// Unlike hack/generate-notes (which generates Go *from* hand-written
// markdown, because the markdown is the only place that content exists),
// this generates markdown *from* Go doc comments, because the content
// already exists there — the struct's own doc comment carries the worked
// examples, and each field's doc comment explains what it does. Hand-writing
// a parallel doc page would just be a second copy that drifts from the
// struct the first time someone adds a field and forgets the docs.
//
// Covers every resource kind declarable on HookTemplates
// (pkg/types/types_hook_templates.go).
//
// Usage:
//
//	go run ./hack/generate-resource-docs
package main

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/doc/comment"
	"go/parser"
	"go/printer"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strconv"
	"strings"
)

// kindSpec maps one *TemplateSource struct to its output doc page.
type kindSpec struct {
	SourceFile string // path relative to repo root
	StructName string // e.g. "DeploymentTemplateSource"
	YAMLKey    string // e.g. "deployments" — the onCreate/onReconcile/onDelete key
	KindName   string // e.g. "Deployment" — the Kubernetes Kind, for the page title
	OutFile    string // e.g. "deployments.md"
	NoDrift    bool   // true if this kind never drift-corrects — reconcile: true has no effect (Job)
}

var kinds = []kindSpec{
	{SourceFile: "pkg/types/types_deployment.go", StructName: "DeploymentTemplateSource", YAMLKey: "deployments", KindName: "Deployment", OutFile: "deployments.md"},
	{SourceFile: "pkg/types/types_replicaset.go", StructName: "ReplicaSetTemplateSource", YAMLKey: "replicaSets", KindName: "ReplicaSet", OutFile: "replicasets.md"},
	{SourceFile: "pkg/types/types_service.go", StructName: "ServiceTemplateSource", YAMLKey: "services", KindName: "Service", OutFile: "services.md"},
	{SourceFile: "pkg/types/types_pod.go", StructName: "PodTemplateSource", YAMLKey: "pods", KindName: "Pod", OutFile: "pods.md"},
	{SourceFile: "pkg/types/types_job.go", StructName: "JobTemplateSource", YAMLKey: "jobs", KindName: "Job", OutFile: "jobs.md", NoDrift: true},
	{SourceFile: "pkg/types/types_job.go", StructName: "CronJobTemplateSource", YAMLKey: "cronJobs", KindName: "CronJob", OutFile: "cronjobs.md"},
	{SourceFile: "pkg/types/types_secret.go", StructName: "SecretTemplateSource", YAMLKey: "secrets", KindName: "Secret", OutFile: "secrets.md"},
	{SourceFile: "pkg/types/types_configmap.go", StructName: "ConfigMapTemplateSource", YAMLKey: "configMaps", KindName: "ConfigMap", OutFile: "configmaps.md"},
	{SourceFile: "pkg/types/types_serviceaccount.go", StructName: "ServiceAccountTemplateSource", YAMLKey: "serviceAccounts", KindName: "ServiceAccount", OutFile: "serviceaccounts.md"},
	{SourceFile: "pkg/types/types_statefulset.go", StructName: "StatefulSetTemplateSource", YAMLKey: "statefulSets", KindName: "StatefulSet", OutFile: "statefulsets.md"},
	{SourceFile: "pkg/types/types_ingress.go", StructName: "IngressTemplateSource", YAMLKey: "ingresses", KindName: "Ingress", OutFile: "ingresses.md"},
	{SourceFile: "pkg/types/types_pvc.go", StructName: "PVTemplateSource", YAMLKey: "persistentVolumes", KindName: "PersistentVolume", OutFile: "persistentvolumes.md"},
	{SourceFile: "pkg/types/types_pvc.go", StructName: "PVCTemplateSource", YAMLKey: "persistentVolumeClaims", KindName: "PersistentVolumeClaim", OutFile: "persistentvolumeclaims.md"},
	{SourceFile: "pkg/types/types_hpa.go", StructName: "HPATemplateSource", YAMLKey: "hpa", KindName: "HorizontalPodAutoscaler", OutFile: "horizontalpodautoscalers.md"},
	{SourceFile: "pkg/types/types_pdb.go", StructName: "PDBTemplateSource", YAMLKey: "pdb", KindName: "PodDisruptionBudget", OutFile: "poddisruptionbudgets.md"},
	{SourceFile: "pkg/types/types_serviceaccount.go", StructName: "NamespaceTemplateSource", YAMLKey: "namespaces", KindName: "Namespace", OutFile: "namespaces.md"},
	{SourceFile: "pkg/types/types_rbac.go", StructName: "RoleTemplateSource", YAMLKey: "roles", KindName: "Role", OutFile: "roles.md"},
	{SourceFile: "pkg/types/types_rbac.go", StructName: "RoleBindingTemplateSource", YAMLKey: "roleBindings", KindName: "RoleBinding", OutFile: "rolebindings.md"},
	{SourceFile: "pkg/types/types_rbac.go", StructName: "ClusterRoleTemplateSource", YAMLKey: "clusterRoles", KindName: "ClusterRole", OutFile: "clusterroles.md"},
	{SourceFile: "pkg/types/types_rbac.go", StructName: "ClusterRoleBindingTemplateSource", YAMLKey: "clusterRoleBindings", KindName: "ClusterRoleBinding", OutFile: "clusterrolebindings.md"},
	{SourceFile: "pkg/types/types_limitrange.go", StructName: "LimitRangeTemplateSource", YAMLKey: "limitRanges", KindName: "LimitRange", OutFile: "limitranges.md"},
	{SourceFile: "pkg/types/types_resourcequota.go", StructName: "ResourceQuotaTemplateSource", YAMLKey: "resourceQuotas", KindName: "ResourceQuota", OutFile: "resourcequotas.md"},
	{SourceFile: "pkg/types/types_networkpolicy.go", StructName: "NetworkPolicyTemplateSource", YAMLKey: "networkPolicies", KindName: "NetworkPolicy", OutFile: "networkpolicies.md"},
	{SourceFile: "pkg/types/custom_resource.go", StructName: "CustomResourceTemplateSource", YAMLKey: "custom", KindName: "Custom Resource", OutFile: "custom.md"},
}

type fieldInfo struct {
	Name    string
	YAMLKey string
	GoType  string
	DescMD  string
}

func main() {
	outDir := "docs/reference/schema/resources"
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		fatalf("creating output dir: %v", err)
	}

	for _, k := range kinds {
		page, err := renderKindPage(k)
		if err != nil {
			fatalf("rendering %s: %v", k.StructName, err)
		}
		outPath := filepath.Join(outDir, k.OutFile)
		if err := os.WriteFile(outPath, []byte(page), 0o644); err != nil {
			fatalf("writing %s: %v", outPath, err)
		}
		fmt.Printf("generated %s\n", outPath)
	}

	indexPath := filepath.Join(outDir, "index.md")
	if err := os.WriteFile(indexPath, []byte(renderIndex(kinds)), 0o644); err != nil {
		fatalf("writing %s: %v", indexPath, err)
	}
	fmt.Printf("generated %s\n", indexPath)
}

func renderKindPage(k kindSpec) (string, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, k.SourceFile, nil, parser.ParseComments)
	if err != nil {
		return "", fmt.Errorf("parsing %s: %w", k.SourceFile, err)
	}

	doc, structType := findStruct(f, k.StructName)
	if structType == nil {
		return "", fmt.Errorf("struct %s not found in %s", k.StructName, k.SourceFile)
	}

	// Struct doc comments open with "<StructName> declares/represents..." —
	// StructName is a Go identifier a blueprint author never writes or sees
	// (they write the YAML key). Swap it for "This" so the published page
	// doesn't leak internal type names into user-facing docs.
	introMD := strings.ReplaceAll(renderDocText(docText(doc)), k.StructName, "This")

	var fields []fieldInfo
	for _, field := range structType.Fields.List {
		if len(field.Names) == 0 {
			continue // embedded field — not handled by the pilot
		}
		name := field.Names[0].Name
		if !ast.IsExported(name) {
			continue
		}

		tag := ""
		if field.Tag != nil {
			if unquoted, err := strconv.Unquote(field.Tag.Value); err == nil {
				tag = unquoted
			}
		}
		yamlKey := strings.Split(reflect.StructTag(tag).Get("yaml"), ",")[0]
		if yamlKey == "" || yamlKey == "-" {
			continue // not exposed in YAML
		}

		fields = append(fields, fieldInfo{
			Name:    name,
			YAMLKey: yamlKey,
			GoType:  typeString(fset, field.Type),
			DescMD:  renderDocText(docText(field.Doc)),
		})
	}

	return buildPage(k, introMD, fields), nil
}

func findStruct(f *ast.File, name string) (*ast.CommentGroup, *ast.StructType) {
	for _, decl := range f.Decls {
		genDecl, ok := decl.(*ast.GenDecl)
		if !ok || genDecl.Tok != token.TYPE {
			continue
		}
		for _, spec := range genDecl.Specs {
			typeSpec, ok := spec.(*ast.TypeSpec)
			if !ok || typeSpec.Name.Name != name {
				continue
			}
			structType, ok := typeSpec.Type.(*ast.StructType)
			if !ok {
				continue
			}
			doc := genDecl.Doc
			if typeSpec.Doc != nil {
				doc = typeSpec.Doc
			}
			return doc, structType
		}
	}
	return nil, nil
}

func docText(cg *ast.CommentGroup) string {
	if cg == nil {
		return ""
	}
	return cg.Text()
}

// renderDocText converts a Go doc comment's plain text (godoc convention —
// tab-indented lines are preformatted code) into Markdown, using the same
// parser/printer godoc itself uses. Handles paragraphs, code blocks, and
// links without hand-rolled tab detection.
// orderedListEscape matches comment.Printer.Markdown()'s escaped list markers
// at the start of a line, e.g. "1\." or "10\." — CommonMark literal escapes
// the printer emits to prevent unintended list rendering, but since our output
// IS the Markdown target, the backslash shows up verbatim. Strip it.
var orderedListEscape = regexp.MustCompile(`(?m)^(\d+)\\\.`)

func renderDocText(text string) string {
	if strings.TrimSpace(text) == "" {
		return ""
	}
	var p comment.Parser
	doc := p.Parse(text)
	var pr comment.Printer
	md := orderedListEscape.ReplaceAllString(string(pr.Markdown(doc)), "$1.")
	return strings.TrimSpace(fencifyCodeBlocks(md))
}

// fencifyCodeBlocks converts comment.Printer's tab-indented code blocks
// (the only style it emits — no fenced-block option) into fenced ```yaml
// blocks, since every code example in this codebase's doc comments is a
// Blueprint YAML snippet. Leaves everything else untouched.
func fencifyCodeBlocks(md string) string {
	lines := strings.Split(md, "\n")
	var out []string
	inCode := false

	flushClose := func() {
		if inCode {
			out = append(out, "```", "")
			inCode = false
		}
	}

	for _, line := range lines {
		if strings.HasPrefix(line, "\t") {
			if !inCode {
				out = append(out, "```yaml")
				inCode = true
			}
			out = append(out, strings.TrimPrefix(line, "\t"))
			continue
		}
		flushClose()
		out = append(out, line)
	}
	flushClose()

	return collapseBlankLines(strings.Join(out, "\n"))
}

// collapseBlankLines squashes runs of 2+ consecutive blank lines down to
// one — fencifyCodeBlocks' own blank-line insertion can double up with
// blank lines the Markdown printer already emitted around a block.
func collapseBlankLines(md string) string {
	lines := strings.Split(md, "\n")
	var out []string
	for _, line := range lines {
		if strings.TrimSpace(line) == "" && len(out) > 0 && strings.TrimSpace(out[len(out)-1]) == "" {
			continue
		}
		out = append(out, line)
	}
	return strings.Join(out, "\n")
}

func typeString(fset *token.FileSet, expr ast.Expr) string {
	var buf bytes.Buffer
	if err := printer.Fprint(&buf, fset, expr); err != nil {
		return "?"
	}
	return buf.String()
}

// namedYAMLShapes maps named Go types whose underlying YAML shape isn't
// visible from their literal type text (e.g. "Labels" reads like a struct
// name, not a map) to the shape word a blueprint author should see instead.
var namedYAMLShapes = map[string]string{
	"Labels": "map",
}

// yamlTypeLabel translates a field's Go type into the shape a blueprint author
// actually writes in YAML (string, boolean, number, list, map, object).
// Blueprint authors never write Go, so no Go syntax (*, [], map[...], struct
// names) should ever reach the published docs.
func yamlTypeLabel(goType string) string {
	t := strings.TrimPrefix(strings.TrimSpace(goType), "*")

	if label, ok := namedYAMLShapes[t]; ok {
		return label
	}
	switch {
	case strings.HasPrefix(t, "[]"):
		return "list"
	case strings.HasPrefix(t, "map["):
		return "map"
	}

	switch t {
	case "string":
		return "string"
	case "bool":
		return "boolean"
	case "int", "int8", "int16", "int32", "int64",
		"uint", "uint8", "uint16", "uint32", "uint64",
		"float32", "float64":
		return "number"
	}

	return "object"
}

// lifecycleNote documents the onCreate/onReconcile/onDelete/reconcile
// convention identically on every page — this is a runtime behavior shared
// by all resource kinds (with the one noted exception), not something each
// struct's doc comment should explain on its own.
func lifecycleNote(k kindSpec) string {
	var b strings.Builder
	b.WriteString("## Lifecycle\n\n")
	b.WriteString("Declare this resource under `onCreate` for an idempotent, one-time create: ")
	b.WriteString("Inrun creates it on the first reconcile and leaves it untouched afterward. ")

	if k.NoDrift {
		b.WriteString(fmt.Sprintf("%s entries are always a one-time create — `reconcile: true` and `onReconcile` have no effect. ", k.KindName))
		b.WriteString(fmt.Sprintf("%s is also commonly declared under `onDelete`, for cleanup work that must complete before the CR's finalizer is removed.\n\n", k.KindName))
		return b.String()
	}

	b.WriteString("Set `reconcile: true` on the same entry to also apply it as drift correction on every subsequent reconcile. ")
	b.WriteString("This is a shorthand for declaring the identical entry under `onReconcile` as well — there's no need to do both.\n\n")
	b.WriteString("Declare a resource under `onDelete` to run explicit cleanup before the CR's finalizer is removed. ")
	b.WriteString("Most resources need no `onDelete` entry — they are garbage-collected automatically through owner references when the CR itself is deleted.\n\n")
	return b.String()
}

func buildPage(k kindSpec, introMD string, fields []fieldInfo) string {
	var b strings.Builder

	fmt.Fprintf(&b, "# %s\n\n", k.KindName)
	if introMD != "" {
		b.WriteString(introMD)
		b.WriteString("\n\n")
	}

	b.WriteString("---\n\n")
	b.WriteString(lifecycleNote(k))

	b.WriteString("---\n\n## Fields\n\n")
	for _, fld := range fields {
		fmt.Fprintf(&b, "### `%s`\n\n", fld.YAMLKey)
		fmt.Fprintf(&b, "Type: %s\n\n", yamlTypeLabel(fld.GoType))
		if fld.DescMD != "" {
			b.WriteString(fld.DescMD)
			b.WriteString("\n\n")
		}
		b.WriteString("---\n\n")
	}

	b.WriteString("## Quick reference\n\n")
	b.WriteString("| YAML key | Type |\n|---|---|\n")
	for _, fld := range fields {
		fmt.Fprintf(&b, "| `%s` | %s |\n", fld.YAMLKey, yamlTypeLabel(fld.GoType))
	}

	return b.String()
}

func renderIndex(kinds []kindSpec) string {
	var b strings.Builder
	b.WriteString("# Resources\n\n")
	b.WriteString("Kubernetes built-ins and custom resources declarable under `onCreate`, `onReconcile`, and `onDelete` in a Blueprint. Each page documents one resource kind's full set of fields — the same schema is reused across all three lifecycle blocks, so a resource's fields don't change depending on which one it's declared under.\n\n")
	b.WriteString("## Reference\n\n")
	b.WriteString("| Kind | YAML key |\n|---|---|\n")
	for _, k := range kinds {
		fmt.Fprintf(&b, "| [%s](%s) | `%s` |\n", k.KindName, k.OutFile, k.YAMLKey)
	}
	return b.String()
}

func fatalf(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, "generate-resource-docs: "+format+"\n", args...)
	os.Exit(1)
}
