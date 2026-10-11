package web

import "testing"

func TestSummaryFor(t *testing.T) {
	cat := &CatalogResponse{CRDs: []CRDSummary{
		{Name: "AppRequest"},
		{Name: "platformresource"},
	}}

	t.Run("nil catalog returns nil", func(t *testing.T) {
		if got := summaryFor(nil, "anything"); got != nil {
			t.Errorf("got %v, want nil", got)
		}
	})

	t.Run("case-insensitive match", func(t *testing.T) {
		got := summaryFor(cat, "apprequest")
		if got == nil || got.Name != "AppRequest" {
			t.Errorf("got %v, want the AppRequest summary", got)
		}
	})

	t.Run("not found returns nil", func(t *testing.T) {
		if got := summaryFor(cat, "nope"); got != nil {
			t.Errorf("got %v, want nil", got)
		}
	})
}

func TestSynthHealthFromSummary(t *testing.T) {
	t.Run("nil summary gives a started placeholder", func(t *testing.T) {
		h := synthHealthFromSummary("foo", nil)
		if h.Name != "foo" || h.State != "started" || !h.Started {
			t.Errorf("got %+v, want a started placeholder named foo", h)
		}
	})

	t.Run("carries fields through from the summary", func(t *testing.T) {
		s := &CRDSummary{
			State: "healthy", Healthy: true, Started: true, Pending: false,
			Uptime: "5m", ErrorRate: 1.5, QueueDepth: 3,
		}
		h := synthHealthFromSummary("bar", s)
		if h.Name != "bar" || h.State != "healthy" || !h.Healthy || h.Uptime != "5m" || h.ErrorRate != 1.5 || h.QueueDepth != 3 {
			t.Errorf("got %+v, did not carry summary fields through", h)
		}
	})
}

func TestSynthInfoFromSummary(t *testing.T) {
	t.Run("nil summary gives a name-only placeholder", func(t *testing.T) {
		i := synthInfoFromSummary("foo", nil)
		if i.Name != "foo" || i.Description != "" {
			t.Errorf("got %+v, want name-only placeholder", i)
		}
	})

	t.Run("carries fields through from the summary", func(t *testing.T) {
		s := &CRDSummary{
			Description: "desc", Mode: "dynamic", GVK: "g/v, Kind=K", GVR: "g/v, Resource=r",
			Namespaced: true, Namespace: "default", Workers: 3, WorkersActive: 2,
			QueueDepth: 1, MaxDepth: 100, ResourceCount: 5, ErrorRate: 0.5,
		}
		i := synthInfoFromSummary("bar", s)
		if i.Name != "bar" || i.Description != "desc" || i.Workers != 3 || i.WorkersActive != 2 || i.ResourceCount != 5 {
			t.Errorf("got %+v, did not carry summary fields through", i)
		}
	})
}

func TestEndpointInfoFor(t *testing.T) {
	cat := &CatalogResponse{CRDs: []CRDSummary{
		{Name: "AppRequest", Endpoints: EndpointInfo{HealthEnabled: false, InfoEnabled: true}},
	}}

	t.Run("nil catalog defaults to fully enabled", func(t *testing.T) {
		got := endpointInfoFor(nil, "AppRequest")
		if !got.HealthEnabled || !got.InfoEnabled {
			t.Errorf("got %+v, want both enabled by default", got)
		}
	})

	t.Run("not found defaults to fully enabled", func(t *testing.T) {
		got := endpointInfoFor(cat, "nope")
		if !got.HealthEnabled || !got.InfoEnabled {
			t.Errorf("got %+v, want both enabled by default", got)
		}
	})

	t.Run("found returns the declared endpoints, case-insensitive", func(t *testing.T) {
		got := endpointInfoFor(cat, "apprequest")
		if got.HealthEnabled || !got.InfoEnabled {
			t.Errorf("got %+v, want HealthEnabled=false InfoEnabled=true", got)
		}
	})
}

func TestEncodeInstance(t *testing.T) {
	cases := []struct{ in, want string }{
		{"http://localhost:8080", "localhost-8080"},
		{"https://localhost:8080", "localhost-8080"},
		{"http://localhost:8080/", "localhost-8080"},
		{"inrun.internal:9090", "inrun.internal-9090"},
	}
	for _, c := range cases {
		if got := encodeInstance(c.in); got != c.want {
			t.Errorf("encodeInstance(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
