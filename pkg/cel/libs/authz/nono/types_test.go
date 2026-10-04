package nono

import (
	"testing"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
)

func TestParseRequestSupportsApprovalProtocolV1Capabilities(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		expression string
	}{
		{
			name: "capability",
			body: `{"backend":"kyverno","request":{"capability_type":"capability","request_id":"req-1","session_id":"session-1","child_pid":123,"path":"/workspace/file","access":"ReadWrite","reason":"edit source"}}`,
			expression: `object.request.capability_type == "capability" &&
				object.request.path == "/workspace/file" &&
				object.request.access == "ReadWrite" &&
				object.request.reason == "edit source" &&
				object.request.child_pid == 123`,
		},
		{
			name: "network",
			body: `{"backend":"kyverno","request":{"capability_type":"network","request_id":"req-2","session_id":"session-2","child_pid":456,"host":"example.com","port":443,"protocol":"tcp","resolved_ips":["192.0.2.1"],"reason":null}}`,
			expression: `object.request.capability_type == "network" &&
				object.request.host == "example.com" &&
				object.request.port == 443 &&
				object.request.protocol == "tcp" &&
				object.request.resolved_ips == ["192.0.2.1"] &&
				object.request.reason == ""`,
		},
		{
			name: "endpoint",
			body: `{"backend":"kyverno","request":{"capability_type":"endpoint","request_id":"req-3","session_id":"session-3","child_pid":0,"route_id":"api","upstream":"https://example.com","method":"GET","path":"/v1","rule_label":"allow-read","reason":""}}`,
			expression: `object.request.capability_type == "endpoint" &&
				object.request.route_id == "api" &&
				object.request.upstream == "https://example.com" &&
				object.request.method == "GET" &&
				object.request.path == "/v1" &&
				object.request.rule_label == "allow-read"`,
		},
		{
			name: "command",
			body: `{"backend":"kyverno","request":{"capability_type":"command","request_id":"req-4","session_id":"session-4","child_pid":789,"command":"kubectl","caller":"agent","args":["kubectl","get","pods"],"intercept_rule":"kubectl","reason":"inspect cluster"}}`,
			expression: `object.request.capability_type == "command" &&
				object.request.command == "kubectl" &&
				object.request.caller == "agent" &&
				object.request.intercept_rule == "kubectl" &&
				object.request.reason == "inspect cluster" &&
				object.request.argv() == ["get", "pods"]`,
		},
	}

	env, err := cel.NewEnv(
		cel.Variable("object", RequestEnvelopeType),
		Lib(),
	)
	if err != nil {
		t.Fatalf("create CEL environment: %v", err)
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, err := ParseRequest([]byte(tt.body))
			if err != nil {
				t.Fatalf("parse request: %v", err)
			}

			ast, issues := env.Compile(tt.expression)
			if issues != nil && issues.Err() != nil {
				t.Fatalf("compile expression: %v", issues.Err())
			}
			program, err := env.Program(ast)
			if err != nil {
				t.Fatalf("create CEL program: %v", err)
			}

			result, _, err := program.Eval(map[string]any{"object": req})
			if err != nil {
				t.Fatalf("evaluate expression: %v", err)
			}
			if result != types.True {
				t.Fatalf("expected true, got %v", result)
			}
		})
	}
}
