# kyverno-nono

> **WARNING: ⚠️ kyverno-nono is in early development stage.**

`kyverno-nono` is a Kyverno CEL-based approval webhook for [nono](https://nono.sh) agent sandboxes. It implements [nono approval protocol v1](https://github.com/nolabs-ai/nono/blob/v0.79.0/docs/protocols/approval-webhook-v1.md), evaluates `ValidatingPolicy` resources in evaluation mode `Nono`, and fails closed when no policy grants a request.

## Table of contents

- [How it works](docs/README.md#how-it-works)
- [Quick start](docs/README.md#quick-start)
- [Policy reference](docs/README.md#policy-reference)
- [CEL object model and functions](docs/README.md#cel-object-model)
- [Samples](#samples)
  - [Read-only command approvals](samples/policies/command-readonly.yaml)
  - [Endpoint allowlist](samples/policies/endpoint-allowlist.yaml)
  - [Filesystem capability approvals](samples/policies/capability-guard.yaml)
  - [Network approvals](samples/policies/network-allowlist.yaml)
  - [Session-scoped exception](samples/exceptions/session-break-glass.yaml)
  - [Scoped break-glass grant](samples/policies/command-break-glass-grant.yaml)
  - [All request payloads](samples/requests/)
- [Configuring nono](docs/README.md#configuring-nono)
- [Deployment](docs/README.md#deployment)
- [Architecture](docs/README.md#architecture)

## Build and test

```sh
go build -o ./cmd/kyverno-nono/kyverno-nono ./cmd/kyverno-nono
go test ./...
```

## Container image

Build the minimal, non-root runtime image from the repository root:

```sh
docker build -t kyverno-nono:dev .
```

The multi-stage build compiles a static Go binary and copies only that binary into the Distroless runtime image.

## Run

Run with local policy files and no Kubernetes cluster:

```sh
./cmd/kyverno-nono/kyverno-nono serve \
  --address=:8765 \
  --external-policy-source=file:///path/to/policies
```

Without a Kubernetes configuration, the server runs in standalone mode and loads policies from external file or OCI sources. With a Kubernetes configuration, it can watch `ValidatingPolicy` resources whose `spec.evaluation.mode` is `Nono` and load external sources as well.

Configure the webhook as an approval backend in the nono profile:

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

Set command invocation or endpoint policy actions to `approve` to route selected requests to the backend. Requests not granted by policy are denied.

## Samples

The [samples directory](samples/README.md) contains standalone policies and request payloads for each nono approval capability. Apply the `ValidatingPolicy` samples to a cluster watched by kyverno-nono, or load their directory as an external policy source.

The [Kubernetes deployment sample](samples/deployment/kyverno-nono.yaml) includes the ServiceAccount, read-only RBAC for `ValidatingPolicy` and `PolicyException`, a hardened Deployment, and a ClusterIP Service. Build and load the image into Kind, then apply it:

```sh
docker build -t kyverno-nono:dev .
kind load docker-image kyverno-nono:dev --name kyverno
kubectl apply -f samples/deployment/kyverno-nono.yaml
```

See [the full guide](docs/README.md) for CEL request fields, policy testing, and deployment options.
