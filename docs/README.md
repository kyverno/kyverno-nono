# kyverno-nono

> **WARNING: ⚠️ kyverno-nono is in early development stage.**

A Kyverno approval server for [nono](https://nono.sh) — the kernel-enforced AI agent sandbox. The webhook implements [approval protocol v1](https://github.com/nolabs-ai/nono/blob/v0.79.0/docs/protocols/approval-webhook-v1.md) and supports the request variants in nono v0.79.0.

When an agent running inside a nono sandbox attempts to execute a command or make an HTTP request, nono can be configured to forward an approval request to this server. Decisions are evaluated using Kyverno CEL policies (`ValidatingPolicy` with `evaluation.mode: Nono`). The server is **fail-closed**: a request with no matching policy is denied.

---

For navigation, use the [table of contents in the repository README](../README.md#table-of-contents).

---

## How it works

```
 AI agent (e.g. Claude Code) inside nono sandbox
        │
        └─ executes: kubectl delete namespace production
                         │
               [nono invocation_policy → approval backend]
                         │
                         ▼
        POST http://localhost:8765/approve
        {
          "backend": "kyverno",
          "request": {
            "capability_type": "command",
            "request_id":      "req-abc123",
            "session_id":      "session-xyz",
            "child_pid":       1234,
            "command":         "kubectl",
            "caller":          "claude",
            "args":            ["kubectl", "delete", "namespace", "production"],
            "intercept_rule":  "kubectl"
          }
        }

        ← {"decision": "denied", "reason": "kubectl delete is not allowed"}
```

HTTP endpoint interception works the same way with `capability_type: "endpoint"`:

```json
{
  "backend": "kyverno",
  "request": {
    "capability_type": "endpoint",
    "request_id":      "req-def456",
    "session_id":      "session-xyz",
    "method":          "DELETE",
    "path":            "/api/v1/namespaces/production",
    "upstream":        "https://kube-apiserver:6443",
    "rule_label":      "endpoint_policy.approve[DELETE /api/**]"
  }
}
```

---

## Quick start

### Run the server (no Kubernetes)

```bash
# Build
go build -o ./cmd/kyverno-nono/kyverno-nono ./cmd/kyverno-nono

# Run against local policy files (no cluster needed)
./kyverno-nono serve \
  --address=:8765 \
  --external-policy-source=file:///path/to/policies
```

### Run in-cluster

```bash
# Deploy — watches ValidatingPolicy resources with evaluation.mode=Nono
./cmd/kyverno-nono/kyverno-nono serve --address=:8765
```

### Configure nono to call kyverno-nono

In your nono JSON profile, register the webhook under `command_policies.approval_backends` and select it as the default backend:

```json
{
  "command_policies": {
    "approval_backends": {
      "kyverno": {
        "type": "webhook",
        "url": "http://127.0.0.1:8765/approve",
        "timeout_secs": 5
      }
    },
    "approval_defaults": {
      "backend": "kyverno",
      "timeout_secs": 5
    }
  }
}
```

### Require approval for selected commands

For example, route direct `kubectl` invocations through Kyverno with a default approval policy:

```json
{
  "command_policies": {
    "approval_backends": {
      "kyverno": {
        "type": "webhook",
        "url": "http://127.0.0.1:8765/approve",
        "timeout_secs": 5
      }
    },
    "approval_defaults": { "backend": "kyverno", "timeout_secs": 5 },
    "commands": {
      "kubectl": {
        "from": {
          "session": {
            "invocation_policy": {
              "default": "approve"
            }
          }
        }
      }
    }
  }
}
```

For endpoint approvals, configure the endpoint policy's `approve` action to use the same backend.

With this setup:

- nono forwards `kubectl` requests selected by the command policy to `kyverno-nono`
- kyverno-nono only grants requests that match an explicit `nono.Grant()` policy
- anything else is denied

Endpoint requests are routed separately by endpoint policies. This covers only requests configured for approval, not arbitrary processes outside nono’s hooks.

---

## Policy reference

### CRD

```yaml
apiVersion: policies.kyverno.io/v1
kind: ValidatingPolicy
metadata:
  name: my-nono-policy
spec:
  matchConstraints:    # required by the ValidatingPolicy API
    resourceRules:
      - apiGroups: ["*"]
        apiVersions: ["*"]
        operations: ["CREATE", "UPDATE", "DELETE", "CONNECT"]
        resources: ["*"]
  evaluation:
    mode: Nono          # required — routes to the nono approval engine
  failurePolicy: Fail   # Fail (deny on compile/eval error) or Ignore (allow on error)
  variables:            # optional — precomputed values available as variables.*
    - name: myVar
      expression: "object.request.command"
  validations:
    - expression: "..."   # must return nono.Grant().Response() or nono.Deny('reason').Response()
      message: "..."      # used in audit logs and PolicyReports
```

### CEL object model

Every policy expression receives a single `object` variable representing the nono approval request.

#### `object` (CheckRequest)

| Field | Type | Description |
|---|---|---|
| `object.backend` | `string` | Backend name configured in the nono profile |
| `object.request` | `RequestData` | The capability-specific request data (see below) |

#### `object.request` (RequestData) — common fields

| Field | Type | Description |
|---|---|---|
| `object.request.capability_type` | `string` | `"capability"`, `"network"`, `"command"`, or `"endpoint"` |
| `object.request.request_id` | `string` | Unique request identifier |
| `object.request.session_id` | `string` | Agent session identifier |
| `object.request.child_pid` | `int` | Requesting child process PID |
| `object.request.reason` | `string` | Optional human-readable reason (empty when omitted) |

#### `object.request` — capability fields (`capability_type == "capability"`)

| Field | Type | Description |
|---|---|---|
| `object.request.path` | `string` | Filesystem path being requested |
| `object.request.access` | `string` | Requested access mode: `"Read"`, `"Write"`, or `"ReadWrite"` |

#### `object.request` — network fields (`capability_type == "network"`)

| Field | Type | Description |
|---|---|---|
| `object.request.host` | `string` | Hostname or IP being connected to |
| `object.request.port` | `int` | TCP/UDP port |
| `object.request.protocol` | `string` | `"tcp"` or `"udp"` |
| `object.request.resolved_ips` | `list(string)` | Resolved IP addresses; may be empty |

#### `object.request` — command fields (`capability_type == "command"`)

| Field | Type | Description |
|---|---|---|
| `object.request.command` | `string` | Binary name, e.g. `"kubectl"` |
| `object.request.caller` | `string` | Calling agent name, e.g. `"claude"` |
| `object.request.args` | `list(string)` | Raw args including the executable name at `[0]` |
| `object.request.argv()` | `list(string)` | Args without the executable name at `[0]` |
| `object.request.intercept_rule` | `string` | Name of the nono rule that triggered the request |

#### `object.request` — endpoint fields (`capability_type == "endpoint"`)

| Field | Type | Description |
|---|---|---|
| `object.request.method` | `string` | HTTP method, e.g. `"GET"`, `"DELETE"` |
| `object.request.path` | `string` | Request path, e.g. `"/api/v1/namespaces/prod"` |
| `object.request.route_id` | `string` | nono route identifier |
| `object.request.upstream` | `string` | Upstream base URL, e.g. `"https://kube-apiserver:6443"` |
| `object.request.rule_label` | `string` | Full rule label from the nono profile |

### CEL functions

| Function | Returns | Description |
|---|---|---|
| `nono.Grant()` | `CheckResponseGranted` | Grant decision builder; call `.Response()` in a validation |
| `nono.Deny("reason")` | `CheckResponseDenied` | Deny decision builder; call `.Response()` in a validation |
| `.Response()` | `CheckResponse` | Convert a grant or deny builder to the response expected by the engine |
| `object.request.argv()` | `list(string)` | `args[1:]` — strips the executable name at index 0 |

> **Note:** A policy `expression` must evaluate to `nono.Grant().Response()`, `nono.Deny(...).Response()`, or `null`. A `null` result is treated as "no decision" — the engine moves to the next policy. If no policy produces a decision the server denies (fail-closed).

---

## Sample policies

### Allow read-only kubectl

Permit `kubectl` only with read-only subcommands. Deny everything else.

```yaml
apiVersion: policies.kyverno.io/v1
kind: ValidatingPolicy
metadata:
  name: nono-kubectl-readonly
spec:
  evaluation:
    mode: Nono
  failurePolicy: Fail
  variables:
    - name: readOnlyVerbs
      expression: "['get', 'list', 'describe', 'logs', 'top', 'explain', 'version']"
  validations:
    - expression: >
        object.request.capability_type != "command" ||
        object.request.command != "kubectl"
        ? nono.Grant().Response()
        : object.request.argv().size() > 0 &&
          variables.readOnlyVerbs.exists(v, object.request.argv()[0] == v)
          ? nono.Grant().Response()
          : nono.Deny(
              "kubectl is restricted to read-only subcommands: " +
              variables.readOnlyVerbs.join(", ")
            ).Response()
      message: "kubectl write operations are not allowed"
```

**Allows:**
- `kubectl get pods`
- `kubectl describe deployment/web`
- `kubectl logs my-pod`

**Denies:**
- `kubectl delete namespace production`
- `kubectl scale deployment/web --replicas=0`
- `kubectl apply -f malicious.yaml`

---

### Block dangerous commands

Deny a list of high-risk binaries outright; allow everything else.

```yaml
apiVersion: policies.kyverno.io/v1
kind: ValidatingPolicy
metadata:
  name: nono-block-dangerous-commands
spec:
  evaluation:
    mode: Nono
  failurePolicy: Fail
  variables:
    - name: blocked
      expression: >
        ['rm', 'dd', 'mkfs', 'shred', 'sudo', 'su', 'chmod', 'chown',
         'iptables', 'ip', 'nc', 'ncat', 'socat', 'tcpdump', 'strace']
  validations:
    - expression: >
        object.request.capability_type != "command"
        ? nono.Grant().Response()
        : variables.blocked.exists(b, b == object.request.command)
          ? nono.Deny("'" + object.request.command + "' is not permitted").Response()
          : nono.Grant().Response()
      message: "command blocked by policy"
```

---

### Allow specific callers

Only permit the `claude` agent to run commands. Block other callers (e.g. scripts running inside the sandbox).

```yaml
apiVersion: policies.kyverno.io/v1
kind: ValidatingPolicy
metadata:
  name: nono-trusted-callers-only
spec:
  evaluation:
    mode: Nono
  failurePolicy: Fail
  variables:
    - name: trustedCallers
      expression: "['claude', 'gemini']"
  validations:
    - expression: >
        object.request.capability_type != "command"
        ? nono.Grant().Response()
        : variables.trustedCallers.exists(c, c == object.request.caller)
          ? nono.Grant().Response()
          : nono.Deny(
              "caller '" + object.request.caller + "' is not in the trusted list"
            ).Response()
      message: "untrusted caller"
```

---

### Restrict HTTP methods (endpoint)

For endpoint requests, allow GET/HEAD/OPTIONS but deny mutating methods.

```yaml
apiVersion: policies.kyverno.io/v1
kind: ValidatingPolicy
metadata:
  name: nono-endpoint-readonly
spec:
  evaluation:
    mode: Nono
  failurePolicy: Fail
  variables:
    - name: readOnlyMethods
      expression: "['GET', 'HEAD', 'OPTIONS']"
  validations:
    - expression: >
        object.request.capability_type != "endpoint"
        ? nono.Grant().Response()
        : variables.readOnlyMethods.exists(m, m == object.request.method)
          ? nono.Grant().Response()
          : nono.Deny(
              "HTTP " + object.request.method + " is not allowed; " +
              "permitted methods: " + variables.readOnlyMethods.join(", ")
            ).Response()
      message: "mutating HTTP methods are not allowed"
```

---

### Allowlist internal upstreams

Only allow endpoint requests to known internal services. Block all external upstreams.

```yaml
apiVersion: policies.kyverno.io/v1
kind: ValidatingPolicy
metadata:
  name: nono-internal-upstreams-only
spec:
  evaluation:
    mode: Nono
  failurePolicy: Fail
  variables:
    - name: allowedUpstreams
      expression: >
        ['https://kube-apiserver:6443',
         'http://metrics.monitoring.svc',
         'http://logging.logging.svc',
         'http://internal-api.default.svc']
  validations:
    - expression: >
        object.request.capability_type != "endpoint"
        ? nono.Grant().Response()
        : variables.allowedUpstreams.exists(u, object.request.upstream == u)
          ? nono.Grant().Response()
          : nono.Deny(
              "upstream '" + object.request.upstream +
              "' is not in the allowlist"
            ).Response()
      message: "requests to external upstreams are not allowed"
```

---

### Session-scoped exceptions

Grant a specific session (`break-glass` scenario) full access for a limited time window. All other sessions still go through the normal rules.

```yaml
apiVersion: policies.kyverno.io/v1
kind: ValidatingPolicy
metadata:
  name: nono-break-glass
spec:
  evaluation:
    mode: Nono
  failurePolicy: Ignore   # if this policy errors, fall through to other policies
  validations:
    - expression: >
        object.request.session_id == "breakglass-session-2026-08-02"
        ? nono.Grant().Response()
        : nono.Deny("not a break-glass session").Response()
      message: "break-glass exception"
---
# Combine with a separate restrictive policy to form a deny-by-default system:
#
# Policy evaluation order (first match wins — fail-closed if no match):
#   1. nono-break-glass  →  Grant if session matches
#   2. nono-kubectl-readonly  →  Grant read-only kubectl, deny write kubectl
#   3. nono-block-dangerous-commands  →  Deny blocked binaries
#   4. (no match)  →  server denies automatically
```

---

### Variables and multi-rule policies

Use `variables` for reusable sub-expressions, and multiple `validations` to express layered logic.

```yaml
apiVersion: policies.kyverno.io/v1
kind: ValidatingPolicy
metadata:
  name: nono-layered-policy
spec:
  evaluation:
    mode: Nono
  failurePolicy: Fail
  variables:
    - name: isCommand
      expression: "object.request.capability_type == 'command'"
    - name: isEndpoint
      expression: "object.request.capability_type == 'endpoint'"
    - name: argv
      expression: "object.request.argv()"
    - name: isMutatingKubectl
      expression: >
        variables.isCommand &&
        object.request.command == "kubectl" &&
        ['apply', 'delete', 'scale', 'patch', 'edit',
         'create', 'replace', 'rollout'].exists(v,
           variables.argv.exists(a, a == v))
    - name: isProductionPath
      expression: >
        variables.isEndpoint &&
        (object.request.path.contains('/namespaces/production') ||
         object.request.path.contains('/namespaces/prod'))

  validations:
    # Rule 1: block all kubectl mutations targeting production
    - expression: >
        variables.isMutatingKubectl &&
        variables.argv.exists(a, a.contains('production') || a.contains('/prod'))
        ? nono.Deny("kubectl mutations in production are not allowed").Response()
        : nono.Grant().Response()
      message: "production kubectl mutation blocked"

    # Rule 2: block HTTP mutations to production namespace paths
    - expression: >
        variables.isProductionPath &&
        !['GET', 'HEAD', 'OPTIONS'].exists(m, m == object.request.method)
        ? nono.Deny(
            "HTTP " + object.request.method +
            " to production path '" + object.request.path + "' is not allowed"
          ).Response()
        : nono.Grant().Response()
      message: "production HTTP mutation blocked"
```

## Configuring nono

Use the webhook backend and command-policy configuration above to route nono's `approve` rules to Kyverno. For example, the command's `from.session.invocation_policy.default` can be set to `"approve"`; endpoint rules can select the same backend in their `approve` action.

---

## Deployment

### Flags

| Flag | Default | Description |
|---|---|---|
| `--address` | `:8765` | Listen address for `POST /approve` |
| `--probes-address` | `:8080` | Liveness/readiness probe address |
| `--metrics-address` | `:9082` | Prometheus metrics address |
| `--cert-file` | `` | TLS certificate file (enables HTTPS) |
| `--key-file` | `` | TLS key file |
| `--kube-policy-source` | `true` | Watch Kubernetes for policies; set to `false` to disable the watch |
| `--external-policy-source` | `` | Filesystem or OCI policy source; can be specified multiple times |
| `--events-enabled` | `false` | Emit Kubernetes Events for each decision |

### Kubernetes deployment (in-cluster)

For a local Kind installation, create the cluster (if needed), install Kyverno, build/load the container image, then install the chart:

```bash
kind get clusters | grep -qx kyverno || kind create cluster --name kyverno
helm repo add kyverno https://kyverno.github.io/kyverno/ --force-update
helm repo update
helm upgrade --install kyverno kyverno/kyverno \
  --namespace kyverno --create-namespace --wait --timeout 5m
docker build -t kyverno-nono:dev .
kind load docker-image kyverno-nono:dev --name kyverno
helm upgrade --install kyverno-nono charts/kyverno-nono \
  --namespace kyverno --create-namespace \
  --set image.repository=kyverno-nono \
  --set image.tag=dev \
  --set commonLabels.team=kyverno
kubectl -n kyverno rollout status deployment/kyverno-nono
```

The chart is in [`charts/kyverno-nono`](../charts/kyverno-nono) and includes a ServiceAccount with read-only access to `ValidatingPolicy` and `PolicyException`, plus a non-root Deployment with a read-only filesystem, dropped capabilities, resource limits, health probes, and a ClusterIP Service. The example targets the `kyverno` namespace and local image tag `kyverno-nono:dev`; change these values as needed. `commonLabels.team=kyverno` satisfies clusters that enforce a `team` label; replace or omit it to match your cluster's label policy. The Service endpoint is `http://kyverno-nono.kyverno.svc:8765/approve`. For non-Kind clusters, publish the image to a registry accessible by the cluster. Applying `ValidatingPolicy` samples as Kubernetes resources also requires a Kyverno installation whose admission webhook supports Nono evaluation and CEL functions; on older installations, provide the policy directory as an external source instead.

Published charts are available as OCI artifacts at `oci://ghcr.io/kyverno/charts/kyverno-nono`. Install a specific published version with:

```bash
helm upgrade --install kyverno-nono \
  oci://ghcr.io/kyverno/charts/kyverno-nono \
  --version <chart-version> \
  --namespace kyverno --create-namespace
```

Apply policies by creating `ValidatingPolicy` resources with `evaluation.mode: Nono` in the cluster — the server watches and hot-reloads them automatically.

---

## Architecture

```
nono sandbox
    │  POST /approve
    ▼
┌─────────────────────────────────────────────────┐
│  kyverno-nono server (:8765)                    │
│                                                 │
│  ParseRequest()  →  CheckRequest{               │
│      backend: "kyverno"                         │
│      request: RequestData{                      │
│          capability_type, command, args, ...    │
│      }                                          │
│  }                                              │
│       │                                         │
│       ▼                                         │
│  CEL engine evaluates ValidatingPolicies        │
│  (mode=Nono, field-selected from kube/OCI)      │
│       │                                         │
│       ├─ nono.Grant()  →  {"decision":"granted"}│
│       └─ nono.Deny(r)  →  {"decision":"denied", │
│                             "reason": r}        │
│       │                                         │
│  No match  →  deny (fail-closed)                │
│                                                 │
│  Side-effects: Prometheus metrics, K8s Events,  │
│  OpenReports PolicyReports                      │
└─────────────────────────────────────────────────┘
```
