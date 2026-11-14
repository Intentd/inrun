package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/intentd/inrun/pkg/types"
	"github.com/intentd/tools/kubeclient/kubefake"
)

func tokenSecret(token string) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "inrun-prod", Namespace: "inrun-system"},
		Data:       map[string][]byte{"token": []byte(token + "\n")},
	}
}

func TestSecretToken_CachesThenRereads(t *testing.T) {
	ctx := context.Background()
	cs := kubefake.NewClientset(tokenSecret("tok-1"))
	src := &secretToken{cs: cs, namespace: "inrun-system", name: "inrun-prod", key: "token"}

	got, err := src.get(ctx)
	require.NoError(t, err)
	assert.Equal(t, "tok-1", got, "trimmed")

	_, err = cs.CoreV1().Secrets("inrun-system").Update(ctx, tokenSecret("tok-2"), metav1.UpdateOptions{})
	require.NoError(t, err)
	got, _ = src.get(ctx)
	assert.Equal(t, "tok-1", got, "cached for tokenCacheFor")

	src.read = time.Now().Add(-tokenCacheFor)
	got, _ = src.get(ctx)
	assert.Equal(t, "tok-2", got, "a refreshed token is picked up once the cache is stale")

	require.NoError(t, cs.CoreV1().Secrets("inrun-system").Delete(ctx, "inrun-prod", metav1.DeleteOptions{}))
	src.read = time.Now().Add(-tokenCacheFor)
	got, err = src.get(ctx)
	require.NoError(t, err)
	assert.Equal(t, "tok-2", got, "a failed read keeps the last token")

	_, err = (&secretToken{cs: cs, namespace: "inrun-system", name: "inrun-prod", key: "token"}).get(ctx)
	assert.Error(t, err, "with no token yet, a failed read is an error")
}

func TestBearer_SetsAuthorization(t *testing.T) {
	var auth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { auth = r.Header.Get("Authorization") }))
	defer srv.Close()

	src := &secretToken{cs: kubefake.NewClientset(tokenSecret("tok-1")), namespace: "inrun-system", name: "inrun-prod", key: "token"}
	client := &http.Client{Transport: bearer{src: src, next: http.DefaultTransport}}
	req, _ := http.NewRequest(http.MethodGet, srv.URL, nil)
	resp, err := client.Do(req)
	require.NoError(t, err)
	resp.Body.Close()
	assert.Equal(t, "Bearer tok-1", auth)
	assert.Empty(t, req.Header.Get("Authorization"), "the caller's request is not changed")
}

func TestTokenCredential(t *testing.T) {
	ref, ok := tokenCredential("prod", types.GatewayClusterConfig{
		Endpoint: "https://p:6443",
		TokenRef: &types.KubernetesSecretRef{Name: "inrun-prod", Key: "token"},
	}, "inrun-system")
	require.True(t, ok)
	assert.Equal(t, credentialRef{cluster: "prod", namespace: "inrun-system", name: "inrun-prod"}, ref, "no namespace is the gateway's own")

	_, ok = tokenCredential("prod", types.GatewayClusterConfig{
		Endpoint:  "https://p:6443",
		SecretRef: &types.KubernetesSecretRef{Name: "kubeconfig", Key: "value"},
	}, "inrun-system")
	assert.False(t, ok, "a kubeconfig credential is not renewed")
}

func TestCredentialRefresher_NothingToDo(t *testing.T) {
	r := NewCredentialRefresher(kubefake.NewClientset(), "inrun-system", nil)
	require.NoError(t, r.Start(context.Background()))
	assert.True(t, r.Started())
	r.Shutdown(context.Background())
}

func TestCredentialRefresher_LeavesLongTokens(t *testing.T) {
	ctx := context.Background()
	hub := kubefake.NewClientset(tokenSecret("long"))
	r := NewCredentialRefresher(hub, "inrun-system", &ClusterRegistry{
		credentials: []credentialRef{{cluster: "prod", namespace: "inrun-system", name: "inrun-prod"}},
	})
	r.refreshAll(ctx)
	s, err := hub.CoreV1().Secrets("inrun-system").Get(ctx, "inrun-prod", metav1.GetOptions{})
	require.NoError(t, err)
	assert.Equal(t, "long\n", string(s.Data["token"]), "a token without an expiry is not touched")
	_, err = hub.CoordinationV1().Leases("inrun-system").Get(ctx, types.GatewayCredentialLeaseName, metav1.GetOptions{})
	assert.NoError(t, err, "replicas take turns under the Lease")
}
