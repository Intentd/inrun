//go:build !runtime && !gateway

package scaffold

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/intentd/inrun/cmd/cli/cmdutil"

	"github.com/intentd/inrun/pkg/version"
	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init [project-name]",
	Short: "Initialize a new Inrun operator project",
	Args: func(cmd *cobra.Command, args []string) error {
		list, _ := cmd.Flags().GetBool("list")
		clear, _ := cmd.Flags().GetBool("clear-cache")

		if list || clear {
			return nil
		}

		if len(args) > 1 {
			return fmt.Errorf("too many arguments — usage: inrun init [project-name]")
		}

		return nil
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		// Handle --list
		list, _ := cmd.Flags().GetBool("list")
		if list {
			return listPacks()
		}

		// Handle --clear-cache
		clear, _ := cmd.Flags().GetBool("clear-cache")
		if clear {
			return clearCache()
		}

		// No name → init in current directory, like terraform init
		name := "."
		if len(args) == 1 {
			name = args[0]
		}

		pack, _ := cmd.Flags().GetString("pack")
		refresh, _ := cmd.Flags().GetBool("refresh-cache")

		if pack == "" {
			return initCanonical(name)
		}
		return initProject(name, pack, refresh)
	},
}

func initCanonical(name string) error {
	cmdutil.PrintBanner()
	label := nameLabel(name)
	fmt.Printf("Initialising %s...\n\n", cmdutil.Bold(label))

	steps := []initStep{}
	if !isCurrentDirectory(name) {
		steps = append(steps, initStep{"Creating project folder", func() error { return os.MkdirAll(name, 0755) }})
	}
	steps = append(steps, initStep{"Writing catalog.yaml", func() error { return extractCanonical(name) }})

	if err := runSteps(steps); err != nil {
		return err
	}

	projectPrint := "✓ Project ready: " + label
	if name == "." {
		projectPrint = "✓ Project ready"
	}

	fmt.Printf("\n%s\n\n", cmdutil.Green(projectPrint))
	if !isCurrentDirectory(name) {
		fmt.Printf("  cd %s\n", name)
	}
	fmt.Printf("  inrun\n\n")
	fmt.Println("To explore example packs:")
	fmt.Printf("  inrun init %s --pack declarative\n", label)

	return nil
}

// nameLabel returns the display name for a project path.
// When initialising in the current directory, it uses the directory's base name.
func nameLabel(name string) string {
	if !isCurrentDirectory(name) {
		return name
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "."
	}
	return filepath.Base(cwd)
}

func initProject(name, pack string, refresh bool) error {

	p, ok := GetPack(pack)
	if !ok {
		return fmt.Errorf("unknown pack: %s", pack)
	}
	first := p.First

	cmdutil.PrintBanner()
	label := nameLabel(name)
	ver := version.Version // ldflags

	projectPrint := "✓ Project ready: " + label
	if name == "." {
		projectPrint = "✓ Project ready"
	}

	fmt.Printf("Initialising %s using '%s' example pack...\n\n",
		cmdutil.Bold(name), pack)

	steps := []initStep{}
	if !isCurrentDirectory(name) {
		steps = append(steps, initStep{"Creating project folder", func() error { return os.MkdirAll(name, 0755) }})
	}

	if refresh {
		steps = append(steps,
			initStep{"Downloading example pack", func() error { return downloadExamplePack(name, pack, ver, refresh) }},
			initStep{"Extracting example pack", func() error { return extractExamplePack(name, pack, ver) }},
		)
	} else {
		steps = append(steps,
			initStep{"Extracting example pack", func() error { return extractEmbeddedPack(name, pack) }},
		)
	}

	if err := runSteps(steps); err != nil {
		return err
	}

	fmt.Printf("\n%s\n\n", cmdutil.Green(projectPrint))

	dir := p.Name
	if first != "" {
		dir = dir + "/" + first
	}
	fmt.Println("To run the first example:")
	if !isCurrentDirectory(name) {
		fmt.Printf("  cd %s/%s\n", name, dir)
	} else {
		fmt.Printf("  cd %s\n", dir)
	}

	if p.Name == "declarative" {
		fmt.Printf("  inrun\n\n")
	} else {
		fmt.Printf("  Follow the steps in the README\n\n")
	}

	fmt.Println("Console:")
	fmt.Printf("  inrun console    # open localhost:8081 (username:password → inrun)\n\n")

	return nil
}

func listPacks() error {
	cmdutil.PrintBanner()

	fmt.Printf("Available example packs:\n")

	for _, p := range ListPacks() {
		fmt.Printf("  %-25s → %s\n", p.Name, p.Description)
	}

	fmt.Printf("\nNo pack — canonical hello-website operator:\n")
	fmt.Printf("  inrun init              # init in current directory\n")
	fmt.Printf("  inrun init my-operator  # init in new folder\n\n")
	fmt.Printf("With a pack:\n")
	fmt.Printf("  inrun init --pack <name>\n")
	fmt.Printf("  inrun init my-operator --pack <name>\n")

	return nil
}

func init() {
	cmdutil.RootCmd.AddCommand(initCmd)
	initCmd.Flags().StringP("pack", "p", "", "Example pack to copy into the project (declarative, remote, typed, intent)")
	initCmd.Flags().BoolP("list", "l", false, "List available example packs")
	initCmd.Flags().Bool("clear-cache", false, "Clear cached example packs")
	initCmd.Flags().Bool("refresh-cache", false, "Fetch pack from GitHub Releases instead of using the built-in copy")

	// Shadow global flags so they don't appear under `inrun init`
	cmdutil.ShadowGlobalCommandFlags(initCmd, "file")
}
