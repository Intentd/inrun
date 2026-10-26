package types

import (
	"testing"

	"gopkg.in/yaml.v3"
)

func TestAPIConfigYAML(t *testing.T) {
	input := `
enabled: true
api:
  enabled: true
  auth:
    tokens:
      - name: ci-pipeline
        secretRef:
          name: inrun-apply-token
          key: token
          rotateAfter: 90d
      - name: local-dev
        token: "${INRUN_DEV_TOKEN}"
`
	var gw GatewayConfig
	if err := yaml.Unmarshal([]byte(input), &gw); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if gw.API == nil {
		t.Fatal("API is nil")
	}
	if !gw.API.Enabled {
		t.Error("API.Enabled should be true")
	}
	tokens := gw.API.Auth.Tokens
	if len(tokens) != 2 {
		t.Fatalf("want 2 tokens, got %d", len(tokens))
	}

	sr := tokens[0]
	if sr.Name != "ci-pipeline" {
		t.Errorf("token[0].Name = %q", sr.Name)
	}
	if sr.SecretRef == nil {
		t.Fatal("token[0].SecretRef is nil")
	}
	if sr.SecretRef.Name != "inrun-apply-token" {
		t.Errorf("secretRef.Name = %q", sr.SecretRef.Name)
	}
	if sr.SecretRef.Key != "token" {
		t.Errorf("secretRef.Key = %q", sr.SecretRef.Key)
	}
	if sr.SecretRef.RotateAfter != "90d" {
		t.Errorf("secretRef.RotateAfter = %q", sr.SecretRef.RotateAfter)
	}

	ev := tokens[1]
	if ev.Token != "${INRUN_DEV_TOKEN}" {
		t.Errorf("token[1].Token = %q", ev.Token)
	}
}

func TestServeConfigYAML(t *testing.T) {
	input := `
serve:
  enabled: true
  fields:
    workloadType:
      label: "Workload Type"
      hint: "app or cert"
      order: 1
    team:
      label: "Team"
      placeholder: "team-payments"
      order: 2
`
	var entry CRDEntry
	if err := yaml.Unmarshal([]byte(input), &entry); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if entry.Serve == nil {
		t.Fatal("serve is nil")
	}
	if !entry.Serve.Enabled {
		t.Error("serve.Enabled should be true")
	}
	wt, ok := entry.Serve.Fields["workloadType"]
	if !ok {
		t.Fatal("missing workloadType field")
	}
	if wt.Label != "Workload Type" {
		t.Errorf("label = %q", wt.Label)
	}
	if wt.Order != 1 {
		t.Errorf("order = %d", wt.Order)
	}
	team := entry.Serve.Fields["team"]
	if team.Placeholder != "team-payments" {
		t.Errorf("placeholder = %q", team.Placeholder)
	}
}

func TestServeConfigIgnoreFieldsAndWhen(t *testing.T) {
	input := `
serve:
  enabled: true
  ignore:
    - internalId
    - createdBy
  category: "Compute"
  description: "Self-service application deployment"
  fields:
    workloadType:
      label: "Workload Type"
      order: 1
      category: "Basic"
    repoURL:
      label: "Repository URL"
      order: 2
      when:
        - field: workloadType
          equals: app
    certIssuer:
      label: "Issuer"
      order: 2
      when:
        - field: workloadType
          equals: cert
        - field: workloadType
          notEquals: app
`
	var entry CRDEntry
	if err := yaml.Unmarshal([]byte(input), &entry); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	serve := entry.Serve
	if serve == nil {
		t.Fatal("serve is nil")
	}
	if len(serve.Ignore) != 2 || serve.Ignore[0] != "internalId" {
		t.Errorf("Ignore = %v", serve.Ignore)
	}
	if serve.Category != "Compute" {
		t.Errorf("Category = %q", serve.Category)
	}
	if serve.Description != "Self-service application deployment" {
		t.Errorf("Description = %q", serve.Description)
	}

	wt := serve.Fields["workloadType"]
	if wt.Category != "Basic" {
		t.Errorf("workloadType.Category = %q", wt.Category)
	}
	if len(wt.When) != 0 {
		t.Errorf("workloadType should have no when conditions")
	}

	repo := serve.Fields["repoURL"]
	if len(repo.When) != 1 {
		t.Fatalf("repoURL.When len = %d, want 1", len(repo.When))
	}
	if repo.When[0].Field != "workloadType" || repo.When[0].Equals != "app" {
		t.Errorf("repoURL.When[0] = %+v", repo.When[0])
	}

	cert := serve.Fields["certIssuer"]
	if len(cert.When) != 2 {
		t.Fatalf("certIssuer.When len = %d, want 2", len(cert.When))
	}
	if cert.When[1].NotEquals != "app" {
		t.Errorf("certIssuer.When[1].NotEquals = %q", cert.When[1].NotEquals)
	}

	// time-based when uses the same Condition infrastructure
	inputWithTime := `
serve:
  enabled: true
  fields:
    environment:
      label: "Environment"
      order: 1
    prodDeploy:
      label: "Production Deploy"
      when:
        - time:
            after: "08:00"
            before: "18:00"
          dayOfWeek:
            weekday: true
`
	var entryTime CRDEntry
	if err := yaml.Unmarshal([]byte(inputWithTime), &entryTime); err != nil {
		t.Fatalf("unmarshal time-when: %v", err)
	}
	pd := entryTime.Serve.Fields["prodDeploy"]
	if len(pd.When) != 1 {
		t.Fatalf("prodDeploy.When len = %d, want 1", len(pd.When))
	}
	if pd.When[0].Time == nil || pd.When[0].Time.After != "08:00" {
		t.Errorf("prodDeploy.When[0].Time = %+v", pd.When[0].Time)
	}
	if pd.When[0].DayOfWeek == nil || pd.When[0].DayOfWeek.Weekday == nil {
		t.Errorf("prodDeploy.When[0].DayOfWeek = %+v", pd.When[0].DayOfWeek)
	}
}

func TestServeFieldOrAndDisabled(t *testing.T) {
	input := `
serve:
  enabled: true
  fields:
    environment:
      label: "Environment"
      order: 1
    prodDeploy:
      label: "Production Deploy"
      or:
        - time:
            after: "08:00"
            before: "18:00"
        - dayOfWeek:
            weekday: true
    legacyFeature:
      label: "Legacy Feature"
      disabled: "Deprecated — use NewFeature instead"
`
	var entry CRDEntry
	if err := yaml.Unmarshal([]byte(input), &entry); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	pd := entry.Serve.Fields["prodDeploy"]
	if len(pd.Or) != 2 {
		t.Fatalf("prodDeploy.Or len = %d, want 2", len(pd.Or))
	}
	if pd.Or[0].Time == nil || pd.Or[0].Time.After != "08:00" {
		t.Errorf("prodDeploy.Or[0].Time = %+v", pd.Or[0].Time)
	}
	if pd.Or[1].DayOfWeek == nil || pd.Or[1].DayOfWeek.Weekday == nil {
		t.Errorf("prodDeploy.Or[1].DayOfWeek = %+v", pd.Or[1].DayOfWeek)
	}

	lf := entry.Serve.Fields["legacyFeature"]
	if lf.Disabled != "Deprecated — use NewFeature instead" {
		t.Errorf("legacyFeature.Disabled = %q", lf.Disabled)
	}
}

func TestAPITokenValidation(t *testing.T) {
	cases := []struct {
		name    string
		token   APIToken
		wantSR  bool
		wantEnv bool
	}{
		{"secretRef", APIToken{Name: "x", SecretRef: &APISecretRef{Name: "s", Key: "k"}}, true, false},
		{"envvar", APIToken{Name: "x", Token: "${MY_TOKEN}"}, false, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if (tc.token.SecretRef != nil) != tc.wantSR {
				t.Errorf("SecretRef presence mismatch")
			}
			if (tc.token.Token != "") != tc.wantEnv {
				t.Errorf("Token presence mismatch")
			}
		})
	}
}
