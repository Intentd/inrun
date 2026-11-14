package api

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"k8s.io/client-go/kubernetes"

	"github.com/intentd/inrun/domain"
	"github.com/intentd/inrun/pkg/tools/inruncluster"
	"github.com/intentd/inrun/pkg/types"
	"github.com/intentd/tools/cluster/access"
	kubeutil "github.com/intentd/tools/kube"
	"github.com/intentd/tools/logger"
)

// tokenCacheFor is how long a remote cluster's token is used before its
// Secret is read again, so a refreshed token reaches every replica.
const tokenCacheFor = time.Minute

// secretToken is a remote cluster's bearer token, read from its credential
// Secret on the gateway cluster and cached for tokenCacheFor.
type secretToken struct {
	cs                   kubernetes.Interface
	namespace, name, key string

	mu    sync.Mutex
	token string
	read  time.Time
}

// get returns the token, reading the Secret again once the cache is stale. A
// failed read keeps the last token: the API server rejects it if it is no
// longer valid, and the next request tries again.
func (s *secretToken) get(ctx context.Context) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.token != "" && time.Since(s.read) < tokenCacheFor {
		return s.token, nil
	}
	token, err := kubeutil.ReadSecretKey(ctx, s.cs, s.namespace, s.name, s.key)
	if err != nil {
		if s.token != "" {
			return s.token, nil
		}
		return "", fmt.Errorf("token %s/%s[%s]: %w", s.namespace, s.name, s.key, err)
	}
	s.token, s.read = strings.TrimSpace(token), time.Now()
	return s.token, nil
}

// bearer sets each request's Authorization from src.
type bearer struct {
	src  *secretToken
	next http.RoundTripper
}

func (b bearer) RoundTrip(req *http.Request) (*http.Response, error) {
	token, err := b.src.get(req.Context())
	if err != nil {
		return nil, err
	}
	req = req.Clone(req.Context())
	req.Header.Set("Authorization", "Bearer "+token)
	return b.next.RoundTrip(req)
}

// credentialRef is where a remote cluster's token credential is on the
// gateway cluster.
type credentialRef struct{ cluster, namespace, name string }

// tokenCredential returns where cfg's token is kept, when it is the token form.
func tokenCredential(name string, cfg types.GatewayClusterConfig, ownNS string) (credentialRef, bool) {
	if cfg.CredentialForm() != "token" || cfg.TokenRef == nil {
		return credentialRef{}, false
	}
	ns := cfg.TokenRef.SecretNamespace()
	if ns == "" {
		ns = ownNS
	}
	return credentialRef{cluster: name, namespace: ns, name: cfg.TokenRef.SecretName()}, true
}

// refreshEvery is how often the refresher checks the credentials; a short
// token is renewed half way through its life (at least 10 minutes), so this
// leaves room for several attempts.
const refreshEvery = time.Minute

// CredentialRefresher renews the short tokens of the gateway's remote
// clusters (granted by `inrun clusters access`) before they expire. Replicas
// take turns under a Lease; a credential already renewed by another replica
// is left alone, and a long token is never touched.
type CredentialRefresher struct {
	hub       kubernetes.Interface
	namespace string
	creds     []credentialRef

	cancel  context.CancelFunc
	done    chan struct{}
	started bool
}

var _ domain.Component = (*CredentialRefresher)(nil)

// NewCredentialRefresher returns a refresher for reg's token credentials,
// taking its Lease in namespace on hub.
func NewCredentialRefresher(hub kubernetes.Interface, namespace string, reg *ClusterRegistry) *CredentialRefresher {
	r := &CredentialRefresher{hub: hub, namespace: namespace}
	if reg != nil {
		r.creds = reg.credentials
	}
	return r
}

func (r *CredentialRefresher) Name() string { return "cluster credential refresher" }

// Started reports whether Start has run.
func (r *CredentialRefresher) Started() bool { return r.started }

// Start checks the credentials now and every refreshEvery until Shutdown.
func (r *CredentialRefresher) Start(ctx context.Context) error {
	r.started = true
	if len(r.creds) == 0 {
		return nil
	}
	ctx, r.cancel = context.WithCancel(ctx)
	r.done = make(chan struct{})
	go func() {
		defer close(r.done)
		ticker := time.NewTicker(refreshEvery)
		defer ticker.Stop()
		for {
			r.refreshAll(ctx)
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		}
	}()
	return nil
}

func (r *CredentialRefresher) Shutdown(ctx context.Context) {
	if r.cancel == nil {
		return
	}
	r.cancel()
	select {
	case <-r.done:
	case <-ctx.Done():
	}
}

func (r *CredentialRefresher) refreshAll(ctx context.Context) {
	err := kubeutil.RunExclusively(ctx, r.hub, kubeutil.LeaseOptions{
		Name:          types.GatewayCredentialLeaseName,
		Namespace:     r.namespace,
		Identity:      kubeutil.LeaseIdentity(),
		LeaseDuration: 30 * time.Second,
		RenewDeadline: 20 * time.Second,
		RetryPeriod:   2 * time.Second,
	}, func(ctx context.Context) error {
		for _, c := range r.creds {
			r.refresh(ctx, c)
		}
		return nil
	})
	if err != nil && ctx.Err() == nil {
		logger.FromContext(ctx).Warn().Err(err).Msg("cluster credential refresh: could not take the Lease")
	}
}

func (r *CredentialRefresher) refresh(ctx context.Context, c credentialRef) {
	log := logger.FromContext(ctx)
	res, err := access.Refresh(ctx, r.hub, c.namespace, c.name, access.RefreshOptions{Naming: inruncluster.Naming})
	switch {
	case err != nil:
		log.Error().Err(err).Str("cluster", c.cluster).
			Msg("cluster credential refresh failed; if the token expired, run `inrun clusters access` for this cluster again")
	case res.Renewed:
		log.Info().Str("cluster", c.cluster).Time("expires", res.ExpiresAt).Msg("cluster credential renewed")
	}
}

// CredentialRefresher renews the short tokens of the API's remote clusters,
// taking its Lease in namespace on hub.
func (s *APIServer) CredentialRefresher(hub kubernetes.Interface, namespace string) *CredentialRefresher {
	return NewCredentialRefresher(hub, namespace, s.clusters)
}
