//go:build !runtime && !gateway

package serve

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/intentd/inrun/cmd/cli/cmdutil"

	oidcpkg "github.com/intentd/inrun/pkg/gateway/oidc"
	"github.com/intentd/inrun/pkg/types"
	"github.com/spf13/cobra"
)

const fileToken = "token.jwt"

// ── inrun token ─────────────────────────────────────────────────────────────────

var tokenCmd = &cobra.Command{
	Use:   "token",
	Short: "Inspect and verify Gateway API tokens",
	Long: `Inspect and verify tokens configured in gateway.api.auth.tokens.

Subcommands:
  verify    Verify a JWT against the configured token entries
  probe     Probe the OIDC discovery endpoint for a token entry
  list      List all configured token entries`,
}

// ── inrun token verify ──────────────────────────────────────────────────────────

var tokenVerifyCmd = &cobra.Command{
	Use:   "verify",
	Short: "Verify a JWT against the configured token entries",
	Long: `Verify a JWT against gateway.api.auth.tokens in the catalog.

Local mode (default): loads the catalog, fetches JWKS from the real provider,
verifies the token signature and claims, and shows which entry matched.

Live mode (--api): sends the token to a running gateway and reports accept/reject.
Use inrun proxy to expose the gateway locally first.

Examples:
  inrun token verify
  inrun token verify -f catalog.yaml -t token.jwt
  inrun token verify --api https://gateway.myorg.io -t token.jwt
  inrun token verify --api http://localhost:8443`,
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		tokenFile, _ := cmd.Flags().GetString("token")
		apiURL, _ := cmd.Flags().GetString("api")
		audienceOverride, _ := cmd.Flags().GetString("audience")

		if tokenFile == "" {
			tokenFile = fileToken
		}

		jwt, err := readTokenFile(tokenFile)
		if err != nil {
			return fmt.Errorf("%s reading token file %q: %w", cmdutil.FailureMark(), tokenFile, err)
		}

		if apiURL != "" {
			return runTokenVerifyLive(cmd, apiURL, jwt)
		}
		return runTokenVerifyLocal(cmd, jwt, audienceOverride, tokenFile)
	},
}

func runTokenVerifyLive(cmd *cobra.Command, apiURL, jwt string) error {
	endpoint := strings.TrimRight(apiURL, "/") + "/api/v1/schema"
	req, err := http.NewRequestWithContext(cmd.Context(), http.MethodGet, endpoint, nil)
	if err != nil {
		return fmt.Errorf("%s building request: %w", cmdutil.FailureMark(), err)
	}
	req.Header.Set("Authorization", "Bearer "+jwt)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("%s gateway unreachable at %s: %w", cmdutil.FailureMark(), apiURL, err)
	}
	defer resp.Body.Close()

	fmt.Printf("\n  %s  %s\n\n", cmdutil.Gray("token verify"), cmdutil.Cyan("live"))
	fmt.Printf("  gateway  %s\n", cmdutil.Bold(apiURL))
	fmt.Printf("  issuer   %s\n\n", cmdutil.Dim(issuerFromJWT(jwt)))

	if resp.StatusCode == http.StatusUnauthorized {
		fmt.Printf("  %s %s\n\n", cmdutil.Red("✗"), cmdutil.Red("token rejected (401 Unauthorized)"))
		return fmt.Errorf("token rejected")
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("%s unexpected status %d: %s", cmdutil.FailureMark(), resp.StatusCode, strings.TrimSpace(string(body)))
	}

	fmt.Printf("  %s token accepted\n\n", cmdutil.Green("✓"))
	return nil
}

func runTokenVerifyLocal(cmd *cobra.Command, jwt, audienceOverride, tokenFile string) error {
	iss := issuerFromJWT(jwt)
	if iss == "" {
		return fmt.Errorf("%s %q does not look like a JWT — could not extract iss claim", cmdutil.FailureMark(), tokenFile)
	}

	k, err := cmdutil.BuildCatalog(cmd)
	if err != nil {
		return fmt.Errorf("%s %w", cmdutil.FailureMark(), err)
	}

	if !k.IsGatewayEnabled() || !k.Gateway.HasAPI() || k.Gateway.API.Auth.Empty() {
		return fmt.Errorf("%s no gateway token auth configured in this catalog", cmdutil.FailureMark())
	}

	var candidates []types.APIToken
	for _, t := range k.Gateway.API.Auth.Tokens {
		if t.IsOIDC() && t.OIDCIssuer() == iss {
			candidates = append(candidates, t)
		}
	}

	fmt.Printf("\n  %s  %s\n\n", cmdutil.Gray("token verify"), cmdutil.Cyan("local"))
	fmt.Printf("  token file  %s\n", cmdutil.Bold(tokenFile))
	fmt.Printf("  issuer      %s\n", cmdutil.Bold(iss))

	if len(candidates) == 0 {
		fmt.Printf("\n  %s no token entry with issuer %q\n\n", cmdutil.Red("✗"), iss)
		return fmt.Errorf("no matching token entry")
	}
	fmt.Printf("  candidates  %s\n", cmdutil.Bold(fmt.Sprintf("%d", len(candidates))))

	cache := oidcpkg.NewCache(oidcpkg.DefaultTTL)
	matched := ""

	for _, entry := range candidates {
		fmt.Printf("\n  %s\n", cmdutil.Dim(strings.Repeat("─", 52)))
		fmt.Printf("  %s  %s\n\n", cmdutil.Cyan(cmdutil.Bold(entry.Name)), cmdutil.Dim(entry.OIDCKind()))

		audience := entry.OIDCAudience()
		if audienceOverride != "" {
			audience = audienceOverride
		}

		claims, err := cache.Verify(entry.OIDCIssuer(), entry.OIDCDiscoveryBase(), jwt, audience)
		if err != nil {
			fmt.Printf("  %s %s\n", cmdutil.Red("✗"), cmdutil.Red(err.Error()))
			continue
		}
		fmt.Printf("  %s signature valid\n", cmdutil.Green("✓"))
		fmt.Printf("  %s not expired\n", cmdutil.Green("✓"))
		fmt.Printf("  %s issuer matched\n", cmdutil.Green("✓"))

		if !entry.MatchesOIDCClaims(claims) {
			fmt.Printf("  %s claims did not match allow block\n\n", cmdutil.Red("✗"))
			printClaimsTable(claims)
			continue
		}
		fmt.Printf("  %s claims matched\n\n", cmdutil.Green("✓"))
		printClaimsTable(claims)
		matched = entry.Name
		break
	}

	fmt.Printf("\n  %s\n", cmdutil.Dim(strings.Repeat("─", 52)))
	if matched != "" {
		fmt.Printf("  %s matched: %s\n\n", cmdutil.Green("✓"), cmdutil.Bold(matched))
		return nil
	}
	fmt.Printf("  %s no matching token entry\n\n", cmdutil.Red("✗"))
	return fmt.Errorf("no matching token entry")
}

func printClaimsTable(claims map[string]string) {
	keys := make([]string, 0, len(claims))
	for k := range claims {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	for _, k := range keys {
		fmt.Fprintf(w, "    %s\t%s\n", cmdutil.Dim(k), claims[k])
	}
	w.Flush()
}

// ── inrun token probe ───────────────────────────────────────────────────────────

var tokenProbeCmd = &cobra.Command{
	Use:   "probe",
	Short: "Probe the OIDC discovery endpoint for a token entry",
	Long: `Probe the OIDC discovery endpoint for a configured token entry.

Fetches the discovery document and JWKS, then reports the result. Useful for
confirming a provider endpoint is reachable before deploying — especially for
Vault, which uses a non-standard discovery path.

Example:
  inrun token probe --name vault-ci
  inrun token probe -f catalog.yaml --name gh-ci`,
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		name, _ := cmd.Flags().GetString("name")
		if name == "" {
			return fmt.Errorf("%s --name is required", cmdutil.FailureMark())
		}

		k, err := cmdutil.BuildCatalog(cmd)
		if err != nil {
			return fmt.Errorf("%s %w", cmdutil.FailureMark(), err)
		}

		if !k.IsGatewayEnabled() || !k.Gateway.HasAPI() || k.Gateway.API.Auth.Empty() {
			return fmt.Errorf("%s no gateway token auth configured in this catalog", cmdutil.FailureMark())
		}

		var entry *types.APIToken
		for i, t := range k.Gateway.API.Auth.Tokens {
			if t.Name == name {
				entry = &k.Gateway.API.Auth.Tokens[i]
				break
			}
		}
		if entry == nil {
			return fmt.Errorf("%s token entry %q not found", cmdutil.FailureMark(), name)
		}
		if !entry.IsOIDC() {
			return fmt.Errorf("%s token entry %q is not an OIDC type (kind: static)", cmdutil.FailureMark(), name)
		}

		discoveryBase := entry.OIDCDiscoveryBase()
		discoveryURL := discoveryBase + "/.well-known/openid-configuration"

		fmt.Printf("\n  %s  %s\n\n", cmdutil.Gray("token probe"), cmdutil.Cyan(name))
		fmt.Printf("  kind       %s\n", cmdutil.Bold(entry.OIDCKind()))
		fmt.Printf("  issuer     %s\n", entry.OIDCIssuer())
		fmt.Printf("  discovery  %s\n\n", cmdutil.Dim(discoveryURL))

		jwksURI, err := probeDiscovery(discoveryURL)
		if err != nil {
			fmt.Printf("  %s discovery failed: %s\n\n", cmdutil.Red("✗"), cmdutil.Red(err.Error()))
			return fmt.Errorf("probe failed")
		}
		fmt.Printf("  %s discovery reachable\n", cmdutil.Green("✓"))
		fmt.Printf("  jwks_uri   %s\n\n", cmdutil.Dim(jwksURI))

		keyCount, algs, err := probeJWKS(jwksURI)
		if err != nil {
			fmt.Printf("  %s JWKS fetch failed: %s\n\n", cmdutil.Red("✗"), cmdutil.Red(err.Error()))
			return fmt.Errorf("probe failed")
		}
		fmt.Printf("  %s JWKS reachable\n", cmdutil.Green("✓"))
		fmt.Printf("  keys       %s  (%s)\n\n", cmdutil.Bold(fmt.Sprintf("%d", keyCount)), strings.Join(algs, ", "))

		return nil
	},
}

func probeDiscovery(discoveryURL string) (string, error) {
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(discoveryURL)
	if err != nil {
		return "", fmt.Errorf("GET %s: %w", discoveryURL, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("GET %s: status %d", discoveryURL, resp.StatusCode)
	}
	var doc struct {
		JWKSURI string `json:"jwks_uri"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&doc); err != nil {
		return "", fmt.Errorf("decoding discovery document: %w", err)
	}
	if doc.JWKSURI == "" {
		return "", fmt.Errorf("discovery document missing jwks_uri")
	}
	return doc.JWKSURI, nil
}

func probeJWKS(jwksURI string) (int, []string, error) {
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(jwksURI)
	if err != nil {
		return 0, nil, fmt.Errorf("GET %s: %w", jwksURI, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return 0, nil, fmt.Errorf("GET %s: status %d", jwksURI, resp.StatusCode)
	}
	var ks struct {
		Keys []struct {
			Alg string `json:"alg"`
			Kty string `json:"kty"`
		} `json:"keys"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&ks); err != nil {
		return 0, nil, fmt.Errorf("decoding JWKS: %w", err)
	}
	algs := make([]string, 0, len(ks.Keys))
	for _, k := range ks.Keys {
		if k.Alg != "" {
			algs = append(algs, k.Alg)
		} else {
			algs = append(algs, k.Kty)
		}
	}
	return len(ks.Keys), algs, nil
}

// ── inrun token list ────────────────────────────────────────────────────────────

var tokenListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all configured token entries",
	Long: `List all token entries in gateway.api.auth.tokens.

Shows each token's name, type, provider kind, and allow summary.

Example:
  inrun token list
  inrun token list -f catalog.yaml`,
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		k, err := cmdutil.BuildCatalog(cmd)
		if err != nil {
			return fmt.Errorf("%s %w", cmdutil.FailureMark(), err)
		}

		if !k.IsGatewayEnabled() || !k.Gateway.HasAPI() || k.Gateway.API.Auth.Empty() {
			fmt.Printf("\n  %s no gateway token auth configured\n\n", cmdutil.Dim("—"))
			return nil
		}

		tokens := k.Gateway.API.Auth.Tokens
		fmt.Printf("\n  %s  %s\n\n", cmdutil.Gray("token list"), cmdutil.Dim(fmt.Sprintf("%d entries", len(tokens))))

		type row struct{ name, typ, provider, allow string }
		rows := make([]row, len(tokens))
		wName, wType, wProv := len("NAME"), len("TYPE"), len("PROVIDER")
		for i, t := range tokens {
			typ, provider, allow := tokenListRow(t)
			rows[i] = row{t.Name, typ, provider, allow}
			if len(t.Name) > wName {
				wName = len(t.Name)
			}
			if len(typ) > wType {
				wType = len(typ)
			}
			if len(provider) > wProv {
				wProv = len(provider)
			}
		}

		gap := 4
		pad := func(s string, w int) string { return s + strings.Repeat(" ", w-len(s)) }
		fmt.Printf("  %s%s%s%s\n",
			cmdutil.Bold(pad("NAME", wName+gap)),
			cmdutil.Bold(pad("TYPE", wType+gap)),
			cmdutil.Bold(pad("PROVIDER", wProv+gap)),
			cmdutil.Bold("ALLOW"),
		)
		for _, r := range rows {
			fmt.Printf("  %s%s%s%s\n",
				cmdutil.Cyan(r.name)+strings.Repeat(" ", wName+gap-len(r.name)),
				pad(r.typ, wType+gap),
				pad(r.provider, wProv+gap),
				cmdutil.Dim(r.allow),
			)
		}
		fmt.Println()
		return nil
	},
}

func tokenListRow(t types.APIToken) (typ, provider, allow string) {
	switch {
	case t.GitHubOIDC != nil:
		return "oidc", "github", allowSummaryGitHub(t.GitHubOIDC.Allow)
	case t.GitLabOIDC != nil:
		return "oidc", "gitlab", allowSummaryGitLab(t.GitLabOIDC.Allow)
	case t.VaultOIDC != nil:
		return "oidc", "vault", allowSummaryVault(t.VaultOIDC)
	case t.OIDC != nil:
		return "oidc", "generic", "issuer=" + t.OIDC.Issuer + " " + allowSummaryMap(t.OIDC.Allow)
	case t.SecretRef != nil:
		ra := ""
		if t.SecretRef.RotateAfter != "" {
			ra = "rotateAfter=" + t.SecretRef.RotateAfter
		}
		return "static", "secretRef", ra
	case t.Token != "":
		return "static", "token", "(env var)"
	default:
		return "unknown", "—", "—"
	}
}

func allowSummaryGitHub(a types.GitHubOIDCClaims) string {
	var parts []string
	if a.Repository != "" {
		parts = append(parts, "repository="+a.Repository)
	}
	if a.RepositoryOwner != "" {
		parts = append(parts, "repositoryOwner="+a.RepositoryOwner)
	}
	if a.Ref != "" {
		parts = append(parts, "ref="+a.Ref)
	}
	if a.Workflow != "" {
		parts = append(parts, "workflow="+a.Workflow)
	}
	if a.Environment != "" {
		parts = append(parts, "environment="+a.Environment)
	}
	if a.JobWorkflowRef != "" {
		parts = append(parts, "jobWorkflowRef="+a.JobWorkflowRef)
	}
	return strings.Join(parts, " ")
}

func allowSummaryGitLab(a types.GitLabOIDCClaims) string {
	var parts []string
	if a.NamespacePath != "" {
		parts = append(parts, "namespacePath="+a.NamespacePath)
	}
	if a.RefProtected != "" {
		parts = append(parts, "refProtected="+a.RefProtected)
	}
	if a.Environment != "" {
		parts = append(parts, "environment="+a.Environment)
	}
	return strings.Join(parts, " ")
}

func allowSummaryVault(v *types.VaultOIDC) string {
	parts := []string{"url=" + v.URL}
	if v.Allow.EntityName != "" {
		parts = append(parts, "entityName="+v.Allow.EntityName)
	}
	if v.Allow.EntityID != "" {
		parts = append(parts, "entityID="+v.Allow.EntityID)
	}
	if v.Allow.Namespace != "" {
		parts = append(parts, "namespace="+v.Allow.Namespace)
	}
	for k, val := range v.Allow.Allow {
		parts = append(parts, k+"="+val)
	}
	return strings.Join(parts, " ")
}

func allowSummaryMap(m map[string]string) string {
	if len(m) == 0 {
		return ""
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(m))
	for _, k := range keys {
		parts = append(parts, k+"="+m[k])
	}
	return strings.Join(parts, " ")
}

// ── helpers ───────────────────────────────────────────────────────────────────

func readTokenFile(path string) (string, error) {
	data, err := cmdutil.ReadLocal(path)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}

// issuerFromJWT decodes the iss claim from a JWT without verifying the signature.
// Returns "" if the string is not a valid JWT.
func issuerFromJWT(token string) string {
	iss, _ := oidcpkg.IssuerFromToken(token)
	return iss
}

// ── init ──────────────────────────────────────────────────────────────────────

func init() {
	tokenVerifyCmd.Flags().StringP("token", "t", "", "File containing the JWT (default: token.jwt)")
	tokenVerifyCmd.Flags().String("api", "", "Gateway base URL for live mode (e.g. http://localhost:8080)")
	tokenVerifyCmd.Flags().String("audience", "", "Override audience check (local mode only)")

	tokenProbeCmd.Flags().StringP("name", "n", "", "Token entry name to probe")

	tokenCmd.AddCommand(tokenVerifyCmd)
	tokenCmd.AddCommand(tokenProbeCmd)
	tokenCmd.AddCommand(tokenListCmd)
	cmdutil.RootCmd.AddCommand(tokenCmd)

	cmdutil.ShadowGlobalCommandFlags(tokenCmd)
}
