# Godaddy Webhook Helm Chart

This Helm chart deploys the GoDaddy DNS webhook for cert-manager. The webhook enables cert-manager
to perform DNS-01 challenges against GoDaddy-managed DNS zones, allowing automated issuance and
renewal of ACME certificates.

## Values

The following table lists the configurable parameters of the chart and their default values.

| Key | Default | Description |
| --- | --- | --- |
| `replicaCount`     | `1` | Number of webhook pod replicas. |
| `image.repository` | `quay.io/snowdrop/cert-manager-webhook-godaddy` | Container image repository for the webhook. |
| `image.tag` | `0.6.0` | Container image tag/version. |
| `image.pullPolicy` | `IfNotPresent` | Image pull policy. |
| `pod.securePort` | _(empty)_ | Override secure HTTPS port exposed by the pod (defaults to 443 if unset). |
| `logging.level` | `info` | Log level: `trace`, `debug`, `info`, `warn`, `error`, `fatal`, `panic`. |
| `logging.format` | `color` | Log format: `text`, `color`, or `json`. |
| `logging.timestamp` | `false` | Whether to include timestamps in log output. |
| `groupName` | `acme.mycompany.com` | The API group name used by the webhook; must match the Issuer/ClusterIssuer `groupName`. |
| `certManager.namespace` | `cert-manager` | Namespace where cert-manager is installed. |
| `certManager.serviceAccountName` | `cert-manager` | Service account name used by cert-manager to call the webhook. |
| `imagePullSecrets` | `[]` | List of image pull secrets for private registries. |
| `nameOverride` | `""` | Override for chart name. |
| `fullnameOverride` | `""` | Override for fully qualified release name. |
| `namespaceOverride` | `""` | Deploy all resources into a specific namespace. |
| `service.type` | `ClusterIP` | Kubernetes Service type for the webhook. |
| `service.port` | `443` | Service port for HTTPS traffic. |
| `resources` | `{}` | Resource requests and limits for the webhook pod. |
| `nodeSelector` | `{}` | Node selector labels for pod scheduling. |
| `tolerations` | `[]` | Tolerations for pod scheduling. |
| `affinity` | `{}` | Affinity/anti-affinity rules for pod scheduling. |

## Usage

- Install the chart with default values:

```bash
export DOMAIN=acme.mydomain.com  # replace with your domain
helm install godaddy-webhook deploy/charts/godaddy-webhook --set groupName=$DOMAIN
```

- Install or upgrade with custom values:

```bash
export DOMAIN=acme.mydomain.com  # replace with your domain
helm upgrade --install godaddy-webhook deploy/charts/godaddy-webhook -f my-values.yaml --set groupName=$DOMAIN
```

- You can also use the Helm chart published on gh-pages
```bash
export DOMAIN=acme.mydomain.com  # replace with your domain
helm repo add godaddy-webhook https://snowdrop.github.io/godaddy-webhook
helm install acme-webhook godaddy-webhook/godaddy-webhook -n cert-manager --set groupName=$DOMAIN
```
Ensure your Issuer or ClusterIssuer is configured to use the same `groupName` as specified in this
chart's values, and that cert-manager is installed and running in the cluster.
