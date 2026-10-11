//go:build !runtime && !gateway

package cluster

import (
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/intentd/inrun/cmd/cli/cmdutil"

	"github.com/intentd/inrun/pkg/tools/proxy"
	"github.com/spf13/cobra"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

var proxyCmd = &cobra.Command{
	Use:   "proxy",
	Short: "Forward Inrun service ports to localhost",
	Long: `Forward ports for deployed Inrun components to localhost.

By default all deployed components are forwarded. Use --for to select specific components.

Examples:
  inrun proxy                        # Forward Runtime, Console, and Gateway
  inrun proxy --for console               # Forward Console only
  inrun proxy --for runtime,console       # Forward Runtime and Console
  inrun proxy -n my-platform-ns      # Forward from a custom namespace
  inrun proxy --runtime-port 9090    # Use port 9090 for Runtime instead of 8080`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ns, _ := cmd.Flags().GetString("namespace")
		runtimePort, _ := cmd.Flags().GetInt("runtime-port")
		consolePort, _ := cmd.Flags().GetInt("console-port")
		gatewayPort, _ := cmd.Flags().GetInt("gateway-port")
		devServerPort, _ := cmd.Flags().GetInt("dev-server-port")
		kubeContext, _ := cmd.Flags().GetString("context")

		include, err := proxyComponentsFromFor(cmd)
		if err != nil {
			return err
		}

		cfg, cs, err := buildProxyClient(kubeContext)
		if err != nil {
			return fmt.Errorf("connect to cluster: %w", err)
		}

		var targets []proxy.ForwardTarget
		if include.runtime {
			targets = append(targets, proxy.ForwardTarget{
				Label:     "Runtime",
				Component: proxy.ComponentRuntime,
				Namespace: ns,
				LocalPort: runtimePort,
				Scheme:    "http",
				ViaLease:  true,
			})
		}
		if include.console {
			targets = append(targets, proxy.ForwardTarget{
				Label:     "Console",
				Component: proxy.ComponentConsole,
				Namespace: ns,
				LocalPort: consolePort,
				Scheme:    "http",
			})
		}
		if include.gateway {
			targets = append(targets, proxy.ForwardTarget{
				Label:     "Gateway",
				Component: proxy.ComponentGateway,
				Namespace: ns,
				LocalPort: gatewayPort,
				Scheme:    "http",
			})
		}
		if include.devServer {
			targets = append(targets, proxy.ForwardTarget{
				Label:       "Dev Server",
				ServiceName: proxy.DevServerServiceName,
				Component:   proxy.DevServer,
				Namespace:   ns,
				LocalPort:   devServerPort,
				Scheme:      "http",
			})
		}

		// Port conflict pre-check
		portOK := true
		for _, t := range targets {
			if err := proxy.CheckPort(t.LocalPort); err != nil {
				fmt.Fprintf(os.Stderr, "  %s %-14s port %d in use — use --%s-port to set an alternative\n",
					cmdutil.Red("✗"), t.Label, t.LocalPort, strings.ToLower(t.Component))
				portOK = false
			}
		}
		if !portOK {
			return fmt.Errorf("one or more local ports are already in use")
		}

		fmt.Printf("\n  Connecting to %s...  %s\n\n", ns, cmdutil.Gray("Press Ctrl+C to stop."))

		ctx, cancel := signal.NotifyContext(cmd.Context(), syscall.SIGINT, syscall.SIGTERM)
		defer cancel()

		proxy.RunAll(ctx, cfg, cs, targets, os.Stdout)

		fmt.Println("\n  Disconnected.")
		return nil
	},
}

// proxyIncludes tracks which components the user selected via --for.
type proxyIncludes struct {
	runtime   bool
	console   bool
	gateway   bool
	devServer bool
}

// proxyComponentsFromFor parses the --for flag using the same vocabulary as
// inrun generate bundle --for: runtime (run), gateway (gw), console.
// An absent flag means all three components.
func proxyComponentsFromFor(cmd *cobra.Command) (proxyIncludes, error) {
	forVal, _ := cmd.Flags().GetString("for")
	if forVal == "" {
		return proxyIncludes{runtime: true, console: true, gateway: true, devServer: true}, nil
	}
	var inc proxyIncludes
	var unknown []string
	for _, part := range strings.Split(forVal, ",") {
		name := strings.TrimSpace(strings.ToLower(part))
		if name == "" {
			continue
		}
		switch name {
		case "run", "runtime":
			inc.runtime = true
		case "gw", "gateway":
			inc.gateway = true
		case "console":
			inc.console = true
		case "dev", "devserver", "dev-server":
			inc.devServer = true
		default:
			unknown = append(unknown, part)
		}
	}
	if len(unknown) > 0 {
		return proxyIncludes{}, fmt.Errorf(
			"inrun: unknown --for value(s): %s\n\nValid values are:\n"+
				"  runtime   	(alias: run)          	— reconcilers, leader election\n"+
				"  gateway   	(alias: gw)            	— TLS, admission webhooks\n"+
				"  console   					— console\n\n"+
				"  devServer 	(alias: devserver)		— dev server\n\n"+
				"Example: --for gateway\n"+
				"         --for runtime,console",
			strings.Join(unknown, ", "),
		)
	}
	if !inc.runtime && !inc.console && !inc.gateway && !inc.devServer {
		return proxyIncludes{}, fmt.Errorf(
			"inrun: --for produced an empty component list\n\n" +
				"Valid values are: runtime (run), gateway (gw), console devserver (dev, devserver, dev-server)",
		)
	}
	return inc, nil
}

// buildProxyClient constructs a REST config and typed clientset from the active
// kubeconfig. Respects the --kubeconfig flag (via cfg) and an explicit context override.
func buildProxyClient(kubeContext string) (*rest.Config, kubernetes.Interface, error) {
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	if cmdutil.Cfg != nil && cmdutil.Cfg.Cluster().KubeconfigPath() != "" {
		loadingRules.ExplicitPath = cmdutil.Cfg.Cluster().KubeconfigPath()
	}
	overrides := &clientcmd.ConfigOverrides{}
	if kubeContext != "" {
		overrides.CurrentContext = kubeContext
	}
	cfg, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		loadingRules, overrides,
	).ClientConfig()
	if err != nil {
		return nil, nil, err
	}
	cs, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, nil, err
	}
	return cfg, cs, nil
}

func init() {
	cmdutil.RootCmd.AddCommand(proxyCmd)

	proxyCmd.Flags().StringP("namespace", "n", cmdutil.DefaultNamespace(), "Namespace where Inrun is deployed")
	proxyCmd.Flags().String("for", "", "Comma-separated components to forward: runtime (run), gateway (gw), console")
	proxyCmd.Flags().Int("runtime-port", 8080, "Local port for Runtime")
	proxyCmd.Flags().Int("console-port", 8081, "Local port for Console")
	proxyCmd.Flags().Int("gateway-port", 8443, "Local port for Gateway")
	proxyCmd.Flags().Int("dev-server-port", 9999, "Local port for Dev Server")
	proxyCmd.Flags().String("context", "", "Kubernetes context to use")

	// Shadow global flags so they don't appear under `inrun proxy`
	cmdutil.ShadowGlobalCommandFlags(proxyCmd, "file")
}
