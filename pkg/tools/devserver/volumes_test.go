package devserver

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func resetVolumes() {
	volumesMu.Lock()
	defer volumesMu.Unlock()
	volumes, volumeKeys, volumeSeq, volumeRequests = map[string]*volume{}, map[string]string{}, 0, nil
}

func volumeAPI(t *testing.T) *httptest.Server {
	t.Helper()
	resetVolumes()
	mux := http.NewServeMux()
	mux.HandleFunc("/volumes", volumesHandler)
	mux.HandleFunc("/volumes/", volumeHandler)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func call(t *testing.T, method, url, key string) (int, map[string]interface{}) {
	t.Helper()
	req, err := http.NewRequest(method, url, strings.NewReader(`{"name":"data","sizeGB":10}`))
	require.NoError(t, err)
	if key != "" {
		req.Header.Set("Idempotency-Key", key)
	}
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	var body map[string]interface{}
	_ = json.NewDecoder(resp.Body).Decode(&body)
	return resp.StatusCode, body
}

func TestVolumes_SameKeyCreatesOnce(t *testing.T) {
	srv := volumeAPI(t)
	_, first := call(t, http.MethodPost, srv.URL+"/volumes", "k-1")
	_, retry := call(t, http.MethodPost, srv.URL+"/volumes", "k-1")
	assert.Equal(t, first["id"], retry["id"], "a retried create answers the first volume")
	_, list := call(t, http.MethodGet, srv.URL+"/volumes", "")
	assert.EqualValues(t, 1, list["count"])
}

func TestVolumes_NoKeyDuplicates(t *testing.T) {
	srv := volumeAPI(t)
	call(t, http.MethodPost, srv.URL+"/volumes", "")
	call(t, http.MethodPost, srv.URL+"/volumes", "")
	_, list := call(t, http.MethodGet, srv.URL+"/volumes", "")
	assert.EqualValues(t, 2, list["count"], "without a key the API cannot tell a retry from a new create")
}

func TestVolumes_ReadThenDelete(t *testing.T) {
	srv := volumeAPI(t)
	code, v := call(t, http.MethodPost, srv.URL+"/volumes", "k")
	require.Equal(t, http.StatusCreated, code)
	url := srv.URL + "/volumes/" + v["id"].(string)

	_, got := call(t, http.MethodGet, url, "")
	assert.Equal(t, "provisioning", got["status"])
	_, got = call(t, http.MethodGet, url, "")
	assert.Equal(t, "ready", got["status"])

	code, _ = call(t, http.MethodDelete, url, "")
	assert.Equal(t, http.StatusAccepted, code)
	code, _ = call(t, http.MethodDelete, url, "")
	assert.Equal(t, http.StatusNotFound, code)
	code, _ = call(t, http.MethodGet, url, "")
	assert.Equal(t, http.StatusNotFound, code)
}
