package serve

import (
	"encoding/json"
	"fmt"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/inrundev/inrun/cmd/cli/cmdutil"
)

// narrator reports the play chain's stages. stageNarrator prints them;
// quietNarrator prints nothing, so the chain runs the same steps either way.
type narrator interface {
	stage(n int, name string)
	ok(detail string)
	fail(detail string)
	detail(detail string)
	json(v interface{})
}

type stageNarrator struct{}

func (stageNarrator) stage(n int, name string) { printStage(n, name) }
func (stageNarrator) ok(d string)              { printStageOK(d) }
func (stageNarrator) fail(d string)            { printStageError(d) }
func (stageNarrator) detail(d string)          { printStageDetail(d) }
func (stageNarrator) json(v interface{})       { printIndentedJSON(v) }

type quietNarrator struct{}

func (quietNarrator) stage(int, string) {}
func (quietNarrator) ok(string)         {}
func (quietNarrator) fail(string)       {}
func (quietNarrator) detail(string)     {}
func (quietNarrator) json(interface{})  {}

func printPlayHeader(file, target, token, op string) {
	fmt.Println()
	fmt.Printf("%s  inrun serve play\n", cmdutil.Bold("▶"))
	fmt.Printf("  %s %s\n", cmdutil.Gray("intent:"), cmdutil.Cyan(file))
	fmt.Printf("  %s %s\n", cmdutil.Gray("target:"), cmdutil.Bold(target))
	fmt.Printf("  %s %s\n", cmdutil.Gray("token: "), cmdutil.Bold(token))
	fmt.Printf("  %s %s\n", cmdutil.Gray("op:    "), cmdutil.Bold(op))
	fmt.Println()
}

func printPlayHeaderForSimulate(catalog, intentFile, target, token string, cycles int) {
	fmt.Println()
	fmt.Printf("%s  inrun simulate\n", cmdutil.Bold("▶"))
	fmt.Printf("  %s %s\n", cmdutil.Gray("catalog:"), cmdutil.Cyan(relPath(catalog)))
	fmt.Printf("  %s %s  %s\n", cmdutil.Gray("intent: "), cmdutil.Cyan(relPath(intentFile)),
		cmdutil.Gray(fmt.Sprintf("(target %s, token %s)", target, token)))
	fmt.Printf("  %s %d\n", cmdutil.Gray("cycles: "), cycles)
	fmt.Println()
}

// printPlayed confirms the CR was built from the intent when the stages
// themselves were not printed.
func printPlayed(intentFile string, obj *unstructured.Unstructured) {
	fmt.Printf("  %s played %s → %s/%s in %s   %s\n\n", cmdutil.SuccessMark(),
		relPath(intentFile), obj.GetKind(), obj.GetName(), obj.GetNamespace(),
		cmdutil.Gray("(--verbose for each stage)"))
}

func printPlayHeaderRead(target, token, op, namespace, name string) {
	fmt.Println()
	fmt.Printf("%s  inrun serve play\n", cmdutil.Bold("▶"))
	fmt.Printf("  %s %s\n", cmdutil.Gray("target:   "), cmdutil.Bold(target))
	fmt.Printf("  %s %s\n", cmdutil.Gray("token:    "), cmdutil.Bold(token))
	fmt.Printf("  %s %s\n", cmdutil.Gray("op:       "), cmdutil.Bold(op))
	if namespace != "" {
		fmt.Printf("  %s %s\n", cmdutil.Gray("namespace:"), cmdutil.Bold(namespace))
	}
	if name != "" {
		fmt.Printf("  %s %s\n", cmdutil.Gray("name:     "), cmdutil.Bold(name))
	}
	fmt.Println()
}

func printStage(n int, name string) {
	fmt.Printf("%s  stage %d · %s\n", cmdutil.Cyan("→"), n, cmdutil.Bold(name))
}

func printStageOK(detail string) {
	fmt.Printf("   %s %s\n", cmdutil.SuccessMark(), detail)
}

func printStageError(detail string) {
	fmt.Printf("   %s %s\n", cmdutil.FailureMark(), cmdutil.Red(detail))
}

func printStageDetail(detail string) {
	fmt.Printf("      %s\n", detail)
}

func printIndentedJSON(v interface{}) {
	b, err := json.MarshalIndent(v, "      ", "  ")
	if err != nil {
		fmt.Printf("      %s\n", cmdutil.Gray("(could not marshal)"))
		return
	}
	fmt.Printf("      %s\n", cmdutil.Gray(string(b)))
}
