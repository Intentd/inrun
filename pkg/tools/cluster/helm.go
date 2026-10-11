package cluster

import (
	"fmt"
	"os"
	"os/exec"
)

// InstallOrUpgradeInrun installs or upgrades the Inrun Helm chart
// idempotently. It always runs `helm upgrade --install` so the call is
// safe whether Inrun is already present or not.
//
// chart is a local chart directory (e.g. ./charts/inrun) to test unreleased
// changes; empty installs the published chart, adding and updating the Inrun
// Helm repo first so the index is current. Version may be empty to use the
// latest published chart. Additional Helm flags (e.g. --set, --atomic) are
// passed through args.
func InstallOrUpgradeInrun(chart, version string, valueFiles []string, args ...string) error {
	ref := chart
	if ref == "" {
		_ = exec.Command("helm", "repo", "add", Inrun, InrunChartRepo).Run()
		_ = exec.Command("helm", "repo", "update", Inrun).Run()
		ref = fmt.Sprintf("%s/%s", Inrun, InrunChartName)
	}

	helmArgs := []string{
		"upgrade", "--install",
		Inrun,
		ref,
		"--namespace", InrunNamespace,
		"--create-namespace",
	}
	if version != "" {
		helmArgs = append(helmArgs, "--version", version)
	}
	for _, f := range valueFiles {
		if f != "" {
			helmArgs = append(helmArgs, "-f", f)
		}
	}
	helmArgs = append(helmArgs, args...)

	cmd := exec.Command("helm", helmArgs...)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("installing/upgrading Inrun: %w\n%s", err, out)
	}
	return nil
}

// BuildConsoleValues generates a temporary Helm values file that enables
// the Console ingress for the given hostname.
// The caller is responsible for removing the file when done.
func BuildConsoleValues(host string) (string, error) {
	content := fmt.Sprintf(`console:
  ingress:
    enabled: true
    hosts:
      - host: %s
        paths:
          - path: /
            pathType: Prefix
`, host)
	tmp, err := os.CreateTemp("", "inrun-console-values-*.yaml")
	if err != nil {
		return "", err
	}
	if _, err := tmp.WriteString(content); err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return "", err
	}
	tmp.Close()
	return tmp.Name(), nil
}
