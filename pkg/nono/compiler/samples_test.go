package compiler

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	v1 "github.com/kyverno/api/api/policies.kyverno.io/v1"
	nonolib "github.com/kyverno/kyverno-nono/pkg/cel/libs/authz/nono"
	"sigs.k8s.io/yaml"
)

func TestPolicySamples(t *testing.T) {
	tests := []struct {
		name       string
		policy     string
		request    string
		exception  string
		wantResult string
	}{
		{
			name:       "read-only command grant",
			policy:     "command-readonly.yaml",
			request:    "command-get.json",
			wantResult: "granted",
		},
		{
			name:       "command mutation denied",
			policy:     "command-readonly.yaml",
			request:    "command-delete.json",
			wantResult: "denied",
		},
		{
			name:       "read-only verb later in command arguments does not grant",
			policy:     "command-readonly.yaml",
			request:    "command-delete-with-get-arg.json",
			wantResult: "denied",
		},
		{
			name:       "untrusted command caller denied",
			policy:     "command-readonly.yaml",
			request:    "command-untrusted-caller.json",
			wantResult: "denied",
		},
		{
			name:       "break-glass policy exception skips restrictive policy",
			policy:     "command-readonly.yaml",
			request:    "command-delete-break-glass.json",
			exception:  "session-break-glass.yaml",
			wantResult: "no decision",
		},
		{
			name:       "break-glass scoped policy grants",
			policy:     "command-break-glass-grant.yaml",
			request:    "command-delete-break-glass.json",
			wantResult: "granted",
		},
		{
			name:       "read-only endpoint grant",
			policy:     "endpoint-allowlist.yaml",
			request:    "endpoint-get.json",
			wantResult: "granted",
		},
		{
			name:       "endpoint mutation denied",
			policy:     "endpoint-allowlist.yaml",
			request:    "endpoint-delete.json",
			wantResult: "denied",
		},
		{
			name:       "upstream hostname with allowlisted prefix denied",
			policy:     "endpoint-allowlist.yaml",
			request:    "endpoint-upstream-prefix.json",
			wantResult: "denied",
		},
		{
			name:       "workspace read capability granted",
			policy:     "capability-guard.yaml",
			request:    "capability-read.json",
			wantResult: "granted",
		},
		{
			name:       "workspace write capability denied",
			policy:     "capability-guard.yaml",
			request:    "capability-write.json",
			wantResult: "denied",
		},
		{
			name:       "allowlisted network destination granted",
			policy:     "network-allowlist.yaml",
			request:    "network-https.json",
			wantResult: "granted",
		},
		{
			name:       "unallowlisted network destination denied",
			policy:     "network-allowlist.yaml",
			request:    "network-denied-host.json",
			wantResult: "denied",
		},
	}

	const sampleRoot = "../../../samples"
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			policyBytes, err := os.ReadFile(sampleRoot + "/policies/" + tt.policy)
			if err != nil {
				t.Fatalf("read policy sample: %v", err)
			}
			var policy v1.ValidatingPolicy
			if err := yaml.UnmarshalStrict(policyBytes, &policy); err != nil {
				t.Fatalf("decode policy sample: %v", err)
			}

			requestBytes, err := os.ReadFile(sampleRoot + "/requests/" + tt.request)
			if err != nil {
				t.Fatalf("read request sample: %v", err)
			}
			request, err := nonolib.ParseRequest(requestBytes)
			if err != nil {
				t.Fatalf("decode request sample: %v", err)
			}

			var exceptions []*v1.PolicyException
			if tt.exception != "" {
				exceptionBytes, err := os.ReadFile(sampleRoot + "/exceptions/" + tt.exception)
				if err != nil {
					t.Fatalf("read exception sample: %v", err)
				}
				var exception v1.PolicyException
				if err := yaml.UnmarshalStrict(exceptionBytes, &exception); err != nil {
					t.Fatalf("decode exception sample: %v", err)
				}
				exceptions = append(exceptions, &exception)
			}

			compiled, errs := NewCompiler(nil).Compile(&policy, exceptions)
			if len(errs) != 0 {
				t.Fatalf("compile policy sample: %v", errs.ToAggregate())
			}
			result, err := compiled.Evaluate(context.Background(), nil, &request)
			if err != nil {
				t.Fatalf("evaluate policy sample: %v", err)
			}
			switch tt.wantResult {
			case "no decision":
				if result != nil {
					t.Fatalf("expected no decision, got %#v", result)
				}
			case "granted":
				if result == nil || result.Granted == nil {
					t.Fatalf("expected grant, got %#v", result)
				}
			case "denied":
				if result == nil || result.Denied == nil {
					t.Fatalf("expected denial, got %#v", result)
				}
			default:
				t.Fatalf("unknown expected result %q", tt.wantResult)
			}
		})
	}
}

func TestSampleRequestsAreValidJSON(t *testing.T) {
	entries, err := os.ReadDir("../../../samples/requests")
	if err != nil {
		t.Fatalf("read request samples: %v", err)
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		t.Run(entry.Name(), func(t *testing.T) {
			body, err := os.ReadFile("../../../samples/requests/" + entry.Name())
			if err != nil {
				t.Fatalf("read request sample: %v", err)
			}
			var payload map[string]any
			if err := json.Unmarshal(body, &payload); err != nil {
				t.Fatalf("decode request sample JSON: %v", err)
			}
			if _, err := nonolib.ParseRequest(body); err != nil {
				t.Fatalf("parse request sample: %v", err)
			}
		})
	}
}
