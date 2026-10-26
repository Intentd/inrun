//go:build !runtime && !gateway

package serve

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/intentd/inrun/cmd/cli/cmdutil"

	"github.com/intentd/inrun/pkg/gateway/api"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var serveApplyCmd = &cobra.Command{
	Use:   "apply",
	Short: "Apply an intent or CR to a live gateway",
	Long: `Apply an intent or CR to a live gateway via POST /api/v1/apply.

The file may be a flat intent (target mode) or a full CR (apiVersion + kind).
Both YAML and JSON are accepted. Defaults to intent.yaml or intent.json in the
current directory if --file is not set.

The gateway handles target resolution, token validation, provenance stamping,
and admission. This command only sends the body and prints the response.

Example (intent mode):
  inrun serve apply --api https://gateway.myorg.io --token "$INRUN_TOKEN"

Example (explicit file):
  inrun serve apply -f intent.yaml --api https://gateway.myorg.io --token "$INRUN_TOKEN"

Example (dry run — no CR applied):
  inrun serve apply -f cr.yaml --api https://gateway.myorg.io --token "$INRUN_TOKEN" --dry-run

Example (override routing surface conflict):
  inrun serve apply -f intent.yaml --api https://gateway.myorg.io --token "$INRUN_TOKEN" --override`,
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		file, _ := cmd.Flags().GetString("file")
		apiURL, _ := cmd.Flags().GetString("api")
		token, _ := cmd.Flags().GetString("token")
		dryRun, _ := cmd.Flags().GetBool("dry-run")
		override, _ := cmd.Flags().GetBool("override")

		if file == "" {
			file = resolveDefaultIntentFile()
			if file == "" {
				return fmt.Errorf("%s no intent file found — create intent.yaml or intent.json, or pass --file <file>", cmdutil.FailureMark())
			}
		}

		raw, err := readIntentFile(file)
		if err != nil {
			return fmt.Errorf("%s reading file %q: %w", cmdutil.FailureMark(), file, err)
		}

		body, err := json.Marshal(raw)
		if err != nil {
			return fmt.Errorf("%s marshalling request body: %w", cmdutil.FailureMark(), err)
		}

		endpoint := strings.TrimRight(apiURL, "/") + "/api/v1/apply"
		switch {
		case dryRun && override:
			endpoint += "?dryRun=true&override=true"
		case dryRun:
			endpoint += "?dryRun=true"
		case override:
			endpoint += "?override=true"
		}

		req, err := http.NewRequestWithContext(cmd.Context(), http.MethodPost, endpoint, bytes.NewReader(body))
		if err != nil {
			return fmt.Errorf("%s building request: %w", cmdutil.FailureMark(), err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return fmt.Errorf("%s gateway unreachable at %s: %w", cmdutil.FailureMark(), apiURL, err)
		}
		defer resp.Body.Close()

		respBody, err := io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("%s reading response: %w", cmdutil.FailureMark(), err)
		}

		var result api.ApplyResponse
		if err := json.Unmarshal(respBody, &result); err != nil {
			return fmt.Errorf("%s unexpected response (HTTP %d): %s", cmdutil.FailureMark(), resp.StatusCode, string(respBody))
		}

		verbose, _ := cmdutil.RootCmd.PersistentFlags().GetBool("verbose")
		if verbose {
			var pretty bytes.Buffer
			if err := json.Indent(&pretty, respBody, "  ", "  "); err == nil {
				fmt.Printf("\n%s\n", pretty.String())
			} else {
				fmt.Printf("\n%s\n", string(respBody))
			}
		}

		return printApplyResult(file, result, dryRun)
	},
}

func printApplyResult(file string, r api.ApplyResponse, dryRun bool) error {
	mode := "apply"
	if dryRun {
		mode = "dry-run"
	}
	fmt.Printf("\n  %s %s  %s\n\n", cmdutil.Gray("serve"), cmdutil.Cyan(mode), cmdutil.Gray(file))

	if !r.Accepted {
		fmt.Printf("  %s %s\n", cmdutil.Red("✗"), cmdutil.Red(r.Message))
		for _, v := range r.Violations {
			fmt.Printf("    %s %s  %s\n", cmdutil.Red("↳"), cmdutil.Gray(v.Field), cmdutil.Red(v.Message))
		}
		return fmt.Errorf("%s", cmdutil.Red("apply rejected"))
	}

	ref := r.Kind
	if r.Namespace != "" {
		ref += "  " + cmdutil.Gray(r.Namespace+"/"+r.Name)
	} else if r.Name != "" {
		ref += "  " + cmdutil.Gray(r.Name)
	}
	fmt.Printf("  %s %s\n", cmdutil.Green("✓"), ref)

	for _, w := range r.Warnings {
		fmt.Printf("  %s %s\n", cmdutil.Yellow("warn"), cmdutil.Yellow(w))
	}

	if r.PollURL != "" {
		fmt.Printf("  %s %s\n", cmdutil.Gray("poll:"), cmdutil.Cyan(r.PollURL))
	}

	if r.Payload != nil {
		b, err := yaml.Marshal(r.Payload)
		if err == nil {
			lines := strings.TrimRight(string(b), "\n")
			indented := "  " + strings.ReplaceAll(lines, "\n", "\n  ")
			fmt.Printf("\n%s\n", indented)
		}
	}

	marker := cmdutil.Green("accepted")
	if dryRun {
		marker = cmdutil.Cyan("dry-run accepted")
	}
	fmt.Printf("\n%s\n\n", marker)
	return nil
}

func init() {
	serveApplyCmd.Flags().StringP("file", "f", "", "Intent or CR file to apply (YAML or JSON; default: intent.yaml or intent.json in cwd)")
	serveApplyCmd.Flags().StringP("api", "a", "http://localhost:8080", "Gateway base URL")
	serveApplyCmd.Flags().StringP("token", "t", "", "Bearer token for the gateway")
	serveApplyCmd.Flags().Bool("dry-run", false, "Preview without applying — the CR is not written to the cluster")
	serveApplyCmd.Flags().Bool("override", false, "Override routing surface conflict — allows switching a resource to a different target")

	_ = serveApplyCmd.MarkFlagRequired("token")

	serveCmd.AddCommand(serveApplyCmd)
}
