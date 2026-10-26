//go:build !runtime && !gateway

package registry

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"oras.land/oras-go/v2"
	"oras.land/oras-go/v2/content/file"
	"oras.land/oras-go/v2/registry/remote"
	"oras.land/oras-go/v2/registry/remote/auth"
	"oras.land/oras-go/v2/registry/remote/credentials"
	"oras.land/oras-go/v2/registry/remote/retry"
)

// Credentials are explicit registry credentials for one pull, as an import's
// auth block declares them. Nil means the Docker credential store
// (~/.docker/config.json), the same login inrun push uses.
type Credentials struct {
	Username string
	Password string
	// Token is a bearer or GitHub token.
	Token string
}

// Fetch returns the local cache directory of an OCI artifact,
// ~/.inrun/registry/<host>/<repo>/<tag>/, pulling it there first when it is not
// cached or refresh is set. It is the one place artifacts reach the disk: the
// CLI, the merger (registry and module imports) and the module loader all use
// it, so a pattern's files keep their layout (manifests/, test/) and its
// relative paths resolve against a directory that outlives the call.
//
// The pull lands in a temporary sibling and is renamed into place, so a failed
// or interrupted pull never leaves a partial cache entry.
func Fetch(ctx context.Context, ref *Ref, refresh bool, cred *Credentials) (string, error) {
	dir, err := ref.CachePath()
	if err != nil {
		return "", err
	}
	if !refresh && ref.IsCached() {
		return dir, nil
	}

	parent := filepath.Dir(dir)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return "", fmt.Errorf("creating cache dir: %w", err)
	}
	tmp, err := os.MkdirTemp(parent, "."+filepath.Base(dir)+"-pull-*")
	if err != nil {
		return "", fmt.Errorf("creating cache dir: %w", err)
	}
	defer os.RemoveAll(tmp) // no-op once renamed

	repo, err := remoteRepository(ref, cred)
	if err != nil {
		return "", err
	}
	store, err := file.New(tmp)
	if err != nil {
		return "", fmt.Errorf("creating file store: %w", err)
	}
	_, err = oras.Copy(ctx, repo, ref.Tag, store, ref.Tag, oras.DefaultCopyOptions)
	if cerr := store.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return "", fmt.Errorf("pulling %s: %w", ref.Full, err)
	}

	if err := os.RemoveAll(dir); err != nil {
		return "", fmt.Errorf("replacing cache entry: %w", err)
	}
	if err := os.Rename(tmp, dir); err != nil {
		return "", fmt.Errorf("replacing cache entry: %w", err)
	}
	return dir, nil
}

// remoteRepository opens ref's repository with explicit credentials, or the
// Docker credential store when cred is nil.
func remoteRepository(ref *Ref, cred *Credentials) (*remote.Repository, error) {
	repo, err := remote.NewRepository(ref.Full)
	if err != nil {
		return nil, fmt.Errorf("invalid reference %q: %w", ref.Full, err)
	}
	client := &auth.Client{
		Client:   retry.DefaultClient,
		Cache:    auth.DefaultCache,
		ClientID: "inrun",
	}
	if cred != nil {
		c := auth.Credential{Username: cred.Username, Password: cred.Password, RefreshToken: cred.Token}
		client.Credential = auth.StaticCredential(ref.Registry, c)
	} else if store, err := credentials.NewStoreFromDocker(credentials.StoreOptions{}); err == nil {
		client.Credential = credentials.Credential(store)
	}
	repo.Client = client
	return repo, nil
}
