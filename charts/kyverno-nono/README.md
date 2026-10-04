# kyverno-nono Helm chart

Installs the kyverno-nono approval service, a ClusterIP Service, and (when Kubernetes policy watching is enabled) a ServiceAccount with read-only access to Kyverno `ValidatingPolicy` and `PolicyException` resources.

## Install from this repository

```sh
helm upgrade --install kyverno-nono ./charts/kyverno-nono \
  --namespace kyverno --create-namespace
```

The chart defaults to the image tag in `Chart.yaml`'s `appVersion`. Override `image.repository` and `image.tag` to use a locally built image or a different registry. For the full Kind workflow, see the [root README](../../README.md#install-in-kind-with-helm).

## Install from the OCI registry

```sh
helm upgrade --install kyverno-nono \
  oci://ghcr.io/kyverno/charts/kyverno-nono \
  --version <chart-version> \
  --namespace kyverno --create-namespace
```

## Configuration

| Value | Default | Description |
|---|---|---|
| `replicaCount` | `1` | Number of server replicas |
| `image.repository` | `ghcr.io/kyverno/kyverno-nono` | Container image repository |
| `image.tag` | `""` | Image tag; defaults to chart `appVersion` |
| `image.pullPolicy` | `IfNotPresent` | Kubernetes image pull policy |
| `image.pullSecrets` | `[]` | Image pull secrets |
| `commonLabels` | `{}` | Additional labels applied to chart resources and Pods |
| `serviceAccount.create` | `true` | Create a ServiceAccount |
| `serviceAccount.name` | `""` | Existing or override ServiceAccount name |
| `kubePolicySource.enabled` | `true` | Watch Kubernetes Nono policies and create read-only RBAC |
| `service.type` | `ClusterIP` | Service type |
| `service.port` | `8765` | Approval webhook Service port |
| `resources` | requests 100m/128Mi; limits 1 CPU/512Mi | Container resource requests and limits |
