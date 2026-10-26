package http

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/intentd/inrun/domain"
	"github.com/intentd/inrun/pkg/runtime/reconcilers/remote/contract"
	"github.com/intentd/inrun/pkg/types"
)

func TestToResult(t *testing.T) {
	r := &Reconciler{decl: &types.RemoteReconcilerDeclaration{Endpoint: "http://x"}}
	req := domain.Request{Key: "default/a"}

	cases := []struct {
		name    string
		body    string
		want    domain.Result
		wantErr string
	}{
		{name: "empty ok", body: `{}`, want: domain.Result{}},
		{
			name: "status, requeue and cleanup together",
			body: `{"result":"ok","status":{"phase":"Done"},
			        "requeue":{"after":"60s","reason":"poll"},
			        "cleanup":{"after":"15s","reason":"finished"}}`,
			want: domain.Result{
				Status:  map[string]interface{}{"phase": "Done"},
				Requeue: &domain.Requeue{After: time.Minute, Reason: "poll"},
				Cleanup: &domain.Cleanup{After: 15 * time.Second, Reason: "finished"},
			},
		},
		{
			name: "labels and annotations",
			body: `{"labels":{"tier":"gold"},"annotations":{"example.com/owner":"team-a"}}`,
			want: domain.Result{
				Labels:      map[string]string{"tier": "gold"},
				Annotations: map[string]string{"example.com/owner": "team-a"},
			},
		},
		{name: "requeueAfter shorthand", body: `{"requeueAfter":"30s"}`, want: domain.Result{RequeueAfter: 30 * time.Second}},
		{name: "cleanup now", body: `{"cleanup":{}}`, want: domain.Result{Cleanup: &domain.Cleanup{}}},
		{name: "error", body: `{"result":"error","error":"boom"}`, wantErr: "boom"},
		{name: "error wins over a malformed directive", body: `{"result":"error","error":"boom","requeue":{"after":"soon"}}`, wantErr: "boom"},
		{name: "old requeue result is rejected", body: `{"result":"requeue","requeueAfter":"5s"}`, wantErr: "requeue field"},
		{name: "bad duration", body: `{"requeue":{"after":"soon"}}`, wantErr: "requeue.after"},
		{name: "negative duration", body: `{"cleanup":{"after":"-5s"}}`, wantErr: "must not be negative"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res, err := contract.Decode([]byte(tc.body))
			var got domain.Result
			if err == nil {
				got, err = r.toResult(context.Background(), req, res)
			}
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("err = %v, want it to mention %q", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			gotJSON, _ := json.Marshal(got)
			wantJSON, _ := json.Marshal(tc.want)
			if string(gotJSON) != string(wantJSON) {
				t.Errorf("got  %s\nwant %s", gotJSON, wantJSON)
			}
		})
	}
}

func TestEffectiveRequeue(t *testing.T) {
	r := domain.Result{RequeueAfter: time.Second, Requeue: &domain.Requeue{After: time.Minute, Reason: "x"}}
	if d, reason := r.EffectiveRequeue(); d != time.Minute || reason != "x" {
		t.Errorf("Requeue must win over RequeueAfter: got %v %q", d, reason)
	}
	if d, _ := (domain.Result{RequeueAfter: time.Second}).EffectiveRequeue(); d != time.Second {
		t.Errorf("shorthand: got %v", d)
	}
}
