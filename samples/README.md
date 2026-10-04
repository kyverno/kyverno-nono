# Approval samples

These examples target nono v0.79.0's [approval webhook protocol v1](https://github.com/nolabs-ai/nono/blob/v0.79.0/docs/protocols/approval-webhook-v1.md). Each policy only matches its intended `capability_type`, so unrelated requests can be evaluated by other policies. Any request with no policy decision is denied by the server.

| Feature | Policy | Example request |
|---|---|---|
| Command rules, caller allowlist, variables, and match conditions | [command-readonly.yaml](policies/command-readonly.yaml) | [kubectl get](requests/command-get.json), [kubectl delete](requests/command-delete.json), [untrusted caller](requests/command-untrusted-caller.json) |
| Scoped break-glass grant | [command-break-glass-grant.yaml](policies/command-break-glass-grant.yaml) | [break-glass session](requests/command-delete-break-glass.json) |
| Endpoint method, route, and upstream restrictions | [endpoint-allowlist.yaml](policies/endpoint-allowlist.yaml) | [allowed GET](requests/endpoint-get.json), [denied DELETE](requests/endpoint-delete.json) |
| Filesystem capability path and access-mode restrictions | [capability-guard.yaml](policies/capability-guard.yaml) | [read](requests/capability-read.json), [write](requests/capability-write.json) |
| Network host, port, and protocol restrictions | [network-allowlist.yaml](policies/network-allowlist.yaml) | [allowed HTTPS](requests/network-https.json), [denied host](requests/network-denied-host.json) |
| Session-scoped PolicyException | [session-break-glass.yaml](exceptions/session-break-glass.yaml) | [break-glass session](requests/command-delete-break-glass.json) |

## Container and Kubernetes deployment

- [Dockerfile](../Dockerfile) builds a static binary in a Go build stage and uses a non-root Distroless runtime.
- [kyverno-nono.yaml](deployment/kyverno-nono.yaml) defines the ServiceAccount, read-only ClusterRole/Binding, hardened Deployment, and ClusterIP Service.

For a local Kind cluster, build and load `kyverno-nono:dev` before applying the manifest. Change `namespace: kyverno` and the image reference if your installation uses a different namespace or registry.

Applying these `ValidatingPolicy` samples as cluster resources requires a Kyverno admission webhook that supports Nono evaluation and CEL functions. On older installations, load the policy files as an external policy source instead.

The [request payload directory](requests/) has JSON fixtures for all four protocol variants: `capability`, `network`, `endpoint`, and `command`. The [break-glass exception](exceptions/session-break-glass.yaml) skips only `nono-kubectl-readonly` and only when its CEL match condition matches. That exception does not grant a request by itself: the separate scoped grant policy shows how a matching session may receive a decision. Any request still lacking a grant is denied by the server.

These are examples, not a complete security policy. Adapt allowlists and session IDs to your environment, and ensure nono routes the intended actions to the configured webhook backend.
