//go:build !runtime && !gateway

// package function implements the functions command for browsing and searching the
// template functions available in Blueprint expressions.
package function

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"text/tabwriter"

	"github.com/intentd/inrun/cmd/cli/cmdutil"

	"github.com/intentd/inrun/pkg/merger"
	"github.com/intentd/inrun/pkg/template/funcs"
	"github.com/intentd/inrun/pkg/types"
	"github.com/intentd/kel"
	"github.com/spf13/cobra"
)

var functionsCmd = &cobra.Command{
	Use:   "functions",
	Short: "Browse and search Inrun functions",
	Long: `List, search, and inspect Inrun functions — the template functions available
in every Blueprint expression.

  inrun functions                    list all functions
  inrun functions --domain strings   filter by domain
  inrun functions search <term>      full-text search
  inrun functions show <name>        show full detail for one function`,
	RunE: func(cmd *cobra.Command, args []string) error {
		domain, _ := cmd.Flags().GetString("domain")
		noPager, _ := cmd.Flags().GetBool("no-pager")
		files, _ := cmd.Flags().GetStringSlice("file")
		expanded := cmdutil.ParseFilePaths(files)

		// When -f is provided, show user-defined functions from the Blueprint.
		if len(expanded) > 0 {
			m := merger.New(expanded...)
			if err := m.Merge(); err != nil {
				return fmt.Errorf("loading blueprint: %w", err)
			}
			reg := m.ToFunctions()
			if reg.Empty() {
				fmt.Println("No user-defined functions declared in this Blueprint.")
				return nil
			}
			return printUserFunctionTable(reg, noPager)
		}

		var fns []kel.FunctionInfo
		if domain != "" {
			fns = funcs.Registry().ByDomain(domain)
			if len(fns) == 0 {
				return fmt.Errorf("no functions found for domain %q — run `inrun functions` to see available domains", domain)
			}
		} else {
			fns = funcs.Registry().Functions
		}
		return printFunctionTable(fns, noPager)
	},
}

var functionsSearchCmd = &cobra.Command{
	Use:   "search <term>",
	Short: "Search functions by name, description, or example",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		noPager, _ := cmd.Flags().GetBool("no-pager")
		results := funcs.Registry().Search(args[0])
		if len(results) == 0 {
			fmt.Printf("no functions match %q\n", args[0])
			return nil
		}
		fmt.Printf("Found %d function(s) matching %q:\n\n", len(results), args[0])
		return printFunctionTable(results, noPager)
	},
}

var functionsShowCmd = &cobra.Command{
	Use:   "show <name>",
	Short: "Show full documentation for a single function",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		noPager, _ := cmd.Flags().GetBool("no-pager")
		n, ok := funcs.Registry().Get(args[0])
		if !ok {
			return fmt.Errorf("function %q not found — run `inrun functions search %s` to find similar functions", args[0], args[0])
		}
		return printFunctionDetail(n, noPager)
	},
}

var functionsDomainCmd = &cobra.Command{
	Use:   "domains",
	Short: "List available function domains",
	RunE: func(cmd *cobra.Command, args []string) error {
		reg := funcs.Registry()
		fmt.Println("Available domains:")
		fmt.Println()
		for _, d := range reg.Domains {
			fmt.Printf("  %-16s  %-22s  %d functions\n", d.Name, d.Title, len(reg.ByDomain(d.Name)))
		}
		return nil
	},
}

func init() {
	functionsCmd.Flags().StringP("domain", "d", "", "Filter by domain (e.g. strings, cron, kubernetes)")
	functionsCmd.Flags().Bool("no-pager", false, "Print directly without paging")
	functionsSearchCmd.Flags().Bool("no-pager", false, "Print directly without paging")
	functionsShowCmd.Flags().Bool("no-pager", false, "Print directly without paging")

	functionsCmd.AddCommand(functionsSearchCmd)
	functionsCmd.AddCommand(functionsShowCmd)
	functionsCmd.AddCommand(functionsDomainCmd)
	cmdutil.RootCmd.AddCommand(functionsCmd)

	// Shadow global flags
	cmdutil.ShadowGlobalCommandFlags(functionsCmd)
}

// ── Rendering ─────────────────────────────────────────────────────────────────

func printFunctionTable(functions []kel.FunctionInfo, noPager bool) error {
	var sb strings.Builder
	w := tabwriter.NewWriter(&sb, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "DOMAIN\tNAME\tDESCRIPTION")
	fmt.Fprintln(w, "──────\t────\t───────────")
	for _, n := range functions {
		desc := n.Description
		if len(desc) > 72 {
			desc = desc[:69] + "..."
		}
		fmt.Fprintf(w, "%s\t%s\t%s\n", n.Domain, n.Name, desc)
	}
	w.Flush()
	return page(sb.String(), noPager)
}

func printFunctionDetail(n kel.FunctionInfo, noPager bool) error {
	sep := strings.Repeat("─", 64)
	var sb strings.Builder
	fmt.Fprintln(&sb, sep)
	fmt.Fprintf(&sb, "  %s\n", n.Name)
	fmt.Fprintln(&sb, sep)
	fmt.Fprintf(&sb, "  Domain:      %s\n", n.Domain)
	if n.Description != "" {
		fmt.Fprintf(&sb, "  Description: %s\n", n.Description)
	}
	if len(n.Keywords) > 0 {
		fmt.Fprintf(&sb, "  Keywords:    %s\n", strings.Join(n.Keywords, ", "))
	}
	if len(n.SeeAlso) > 0 {
		fmt.Fprintf(&sb, "  See also:    %s\n", strings.Join(n.SeeAlso, ", "))
	}
	if n.Example != "" {
		fmt.Fprintln(&sb)
		fmt.Fprintln(&sb, "  Example:")
		for _, line := range strings.Split(n.Example, "\n") {
			fmt.Fprintf(&sb, "    %s\n", line)
		}
	}
	fmt.Fprintln(&sb, sep)
	return page(sb.String(), noPager)
}

// page writes output through `less` when stdout is a terminal, otherwise prints directly.
func page(content string, noPager bool) error {
	if noPager || !cmdutil.IsTerminal() {
		fmt.Print(content)
		return nil
	}
	pager := exec.Command("less", "-RFX")
	pager.Stdin = strings.NewReader(content)
	pager.Stdout = os.Stdout
	pager.Stderr = os.Stderr
	if err := pager.Run(); err != nil {
		fmt.Print(content)
	}
	return nil
}

func printUserFunctionTable(reg types.KELFunctionRegistry, noPager bool) error {
	var sb strings.Builder
	w := tabwriter.NewWriter(&sb, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "NAME\tEXPRESSION\tDESCRIPTION")
	fmt.Fprintln(w, "────\t──────────\t───────────")
	for _, n := range reg.Definitions {
		expr := n.Expression
		if len(expr) > 60 {
			expr = expr[:57] + "..."
		}
		desc := n.Description
		if len(desc) > 50 {
			desc = desc[:47] + "..."
		}
		fmt.Fprintf(w, "%s\t%s\t%s\n", n.Name, expr, desc)
	}
	w.Flush()
	return page(sb.String(), noPager)
}
