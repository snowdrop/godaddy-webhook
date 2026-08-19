# How-To: Deploy cert-manager + GoDaddy Webhook on a Local Kind Cluster

This guide walks through deploying cert-manager v1.17.4 and the GoDaddy webhook on a local kind cluster, creating the necessary resources, and running the integration test suite.

## Prerequisites

- [kind](https://kind.sigs.k8s.io/docs/user/quick-start/#installation) installed
- [kubectl](https://kubernetes.io/docs/tasks/tools/) installed
- [Helm](https://helm.sh/docs/intro/install/) installed
- A GoDaddy domain with API access
- A GoDaddy Personal Access Token (PAT) for API v3

## 1. Create a Kind Cluster

```fish
set -x KIND_CLUSTER snowdrop  # choose your cluster name
kind create cluster --name $KIND_CLUSTER
```

Verify the cluster is running:

```fish
kubectl cluster-info --context kind-$KIND_CLUSTER
```

## 2. Deploy cert-manager v1.17.4

```fish
kubectl apply -f https://github.com/jetstack/cert-manager/releases/download/v1.17.4/cert-manager.yaml
```

Wait for cert-manager pods to be ready:

```fish
kubectl wait --for=condition=Ready pods --all -n cert-manager --timeout=120s
```

Verify all three pods are running (cainjector, controller, webhook):

```fish
kubectl get pods -n cert-manager
```

### Fix CoreDNS to resolve external domains

By default, CoreDNS in kind forwards to `/etc/resolv.conf` inside the node container, which may not resolve external domains. Update CoreDNS to forward to public DNS servers:

```fish
kubectl get configmap coredns -n kube-system -o json \
  | jq '.data.Corefile |= gsub("forward \\. /etc/resolv\\.conf"; "forward . 8.8.8.8 1.1.1.1")' \
  | kubectl apply -f -
kubectl rollout restart deployment coredns -n kube-system
```

Verify it works:

```fish
kubectl run dns-test --image=busybox --restart=Never --command -- sh -c "nslookup -type=SOA snowdrop.dev"
sleep 5
kubectl logs dns-test
kubectl delete pod dns-test
```

### Patch cert-manager for DNS01 recursive nameservers

Cert-manager also needs to use public DNS (not CoreDNS) for DNS01 challenge propagation checks:

```fish
kubectl patch deployment cert-manager -n cert-manager --type=json -p '[
 {"op": "add", "path": "/spec/template/spec/containers/0/args/-", "value": "--dns01-recursive-nameservers=8.8.8.8:53,1.1.1.1:53"},
 {"op": "add", "path": "/spec/template/spec/containers/0/args/-", "value": "--dns01-recursive-nameservers-only"}
]'
```

The cert-manager pod will restart automatically.

## 3. Build and Install the GoDaddy Webhook

### Build the image and load it into kind

The Helm chart defaults to a pre-built image from the container registry. To run the latest code (e.g., after local changes), build the image locally and load it into kind.

Kind uses podman as its container runtime. Since `kind load docker-image` does not work with podman, save the image to a tar archive and use `kind load image-archive`:

```fish
podman build -t quay.io/snowdrop/cert-manager-webhook-godaddy:latest .
podman save quay.io/snowdrop/cert-manager-webhook-godaddy:latest -o /tmp/webhook.tar
kind load image-archive /tmp/webhook.tar --name $KIND_CLUSTER
rm /tmp/webhook.tar
```

### Install with Helm

Install the webhook using the Helm chart from the `deploy/` directory. Set `image.tag=latest` and `image.pullPolicy=Never` so kind uses the locally loaded image. The `groupName` is a custom Kubernetes API group name (it does not need to match your domain):

```fish
set -x DOMAIN acme.mydomain.com
helm install -n cert-manager godaddy-webhook ./deploy/charts/godaddy-webhook \
  --set groupName=$DOMAIN \
  --set image.tag=latest \
  --set image.pullPolicy=Never
```

To upgrade after rebuilding the image:

```fish
podman build -t quay.io/snowdrop/cert-manager-webhook-godaddy:latest .
podman save quay.io/snowdrop/cert-manager-webhook-godaddy:latest -o ./webhook.tar
kind load image-archive ./webhook.tar --name $KIND_CLUSTER
rm ./webhook.tar
helm upgrade -n cert-manager godaddy-webhook ./deploy/charts/godaddy-webhook \
  --set groupName=$DOMAIN \
  --set image.tag=latest \
  --set image.pullPolicy=Never
```

### Use a PR image

When a pull request is opened, CI automatically builds a container image tagged `pr-<number>` and posts a comment on the PR with the image reference. To test a PR image on your kind cluster:

```fish
set PR_NUMBER 42  # replace with the actual PR number
set PR_IMAGE quay.io/snowdrop/cert-manager-webhook-godaddy:pr-$PR_NUMBER

podman pull $PR_IMAGE
podman save $PR_IMAGE -o /tmp/webhook-pr.tar
kind load image-archive /tmp/webhook-pr.tar --name $KIND_CLUSTER
rm /tmp/webhook-pr.tar
helm upgrade --install -n cert-manager godaddy-webhook ./deploy/charts/godaddy-webhook \
  --set groupName=$DOMAIN \
  --set image.tag=pr-$PR_NUMBER \
  --set image.pullPolicy=Never
```

Verify the webhook pod is running:

```fish
kubectl get pods -n cert-manager -l app.kubernetes.io/name=godaddy-webhook
```

## 4. Create the Secret with PAT

For API v3, the secret contains a Personal Access Token (not an API key pair).

To create a PAT, go to the [GoDaddy Developer Portal](https://developer.godaddy.com), navigate to API Keys, create a new Personal Access Token with scopes `domains.domain:read` and `domains.dns:update`.

Export your PAT as an environment variable, then create the secret:

```fish
set -x GODADDY_PAT "your-personal-access-token-here"

kubectl apply -n cert-manager -f (echo "
apiVersion: v1
kind: Secret
metadata:
  name: godaddy-api-key
type: Opaque
stringData:
  token: $GODADDY_PAT
" | psub)
```

## 5. Create the ClusterIssuer

```fish
set -x EMAIL "your-email@example.com"
set -x YOUR_DOMAIN "snowdrop.dev"

kubectl apply -f (echo "
apiVersion: cert-manager.io/v1
kind: ClusterIssuer
metadata:
  name: letsencrypt-staging
spec:
  acme:
    server: https://acme-staging-v02.api.letsencrypt.org/directory
    email: $EMAIL
    privateKeySecretRef:
      name: letsencrypt-staging
    solvers:
    - selector:
        dnsZones:
        - '$YOUR_DOMAIN'
      dns01:
        webhook:
          config:
            apiKeySecretRef:
              name: godaddy-api-key
              key: token
            production: true
            apiVersion: 'v3'
            ttl: 600
          groupName: $DOMAIN
          solverName: godaddy
" | psub)
```

## 6. Create a Certificate

```fish
kubectl apply -n cert-manager -f (echo "
apiVersion: cert-manager.io/v1
kind: Certificate
metadata:
  name: wildcard-$YOUR_DOMAIN
spec:
  secretName: wildcard-$YOUR_DOMAIN-tls
  renewBefore: 240h
  dnsNames:
  - '*.$YOUR_DOMAIN'
  issuerRef:
    name: letsencrypt-staging
    kind: ClusterIssuer
" | psub)
```

Monitor the certificate status:

```fish
kubectl get certificate -n cert-manager
kubectl describe certificate wildcard-$YOUR_DOMAIN -n cert-manager
kubectl describe certificaterequest -n cert-manager
kubectl get challenges -n cert-manager
kubectl describe challenge -n cert-manager
```

### Clean up and recreate a Certificate

To start fresh (e.g., after fixing a config issue), delete the certificate and all its dependent resources in order:

```fish
# Delete challenges first, then orders, certificate requests, and finally the certificate
kubectl delete challenges --all -n cert-manager
kubectl delete orders --all -n cert-manager
kubectl delete certificaterequests --all -n cert-manager
kubectl delete certificate wildcard-$YOUR_DOMAIN -n cert-manager

# Delete the associated secret and private key
kubectl delete secret wildcard-$YOUR_DOMAIN-tls -n cert-manager 2>/dev/null
kubectl delete secret (kubectl get secret -n cert-manager -o name | grep wildcard-$YOUR_DOMAIN | head -1) -n cert-manager 2>/dev/null
```

Then recreate the certificate by re-running the `kubectl apply` command from the section above.

## 7. Running `make test` (Integration / DNS Conformance Test)

### What `make test` does

The `make test` target Druns the cert-manager ACME DNS01 conformance test suite against a **real GoDaddy domain**. Here is the sequence:

1. **Clean up**: removes `vendor/`, `_out/`, and `apiserver.local.config/` directories.
2. **Fetch test binaries**: runs `scripts/fetch-test-binaries.sh`, which uses `setup-envtest` to download kubebuilder binaries (`etcd`, `kube-apiserver`, `kubectl`) into `_out/kubebuilder/`.
3. **Start an in-process control plane**: the test launches a local `etcd` and `kube-apiserver` (via the envtest library) -- no full cluster is required.
4. **Run the conformance suite** (`dns_resolver_test.go`):
   - Loads the solver config from `testdata/godaddy/<version>/config.json` and credentials from `testdata/godaddy/<version>/secret.yaml`.
   - Calls `Present()` to create a real TXT record (`cert-manager-dns01-tests`) on your GoDaddy domain via the API.
   - Polls the DNS server (`TEST_DNS_SERVER`) to verify the TXT record propagated.
   - Calls `CleanUp()` to delete the TXT record via the API.
   - Polls the DNS server again to verify the record was removed.

### Environment variables

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `TEST_ZONE_NAME` | Yes | `example.com.` | Your GoDaddy domain (must end with a dot) |
| `TEST_DNS_SERVER` | No | `1.1.1.1:53` | DNS server to query for propagation checks. Use your domain's authoritative nameserver for best results. |
| `TEST_API_VERSION` | No | `v1` | GoDaddy API version to test (`v1` or `v3`) |
| `TEST_TIMEOUT` | No | `3m` | Maximum time to wait for DNS propagation |

### Credential setup

Before running the test, make sure the credential secret file exists for your API version:

```fish
# For API v3: write the secret file with your PAT
echo "
apiVersion: v1
kind: Secret
metadata:
  name: godaddy-credentials
type: Opaque
stringData:
  token: $GODADDY_PAT
" > testdata/godaddy/v3/secret.yaml
```

### Find your authoritative nameserver

```fish
dig NS $YOUR_DOMAIN +short
# Example output: ns33.domaincontrol.com.
```

### Run the test

```fish
# API v1 (default)
make test TEST_ZONE_NAME=snowdrop.dev. TEST_DNS_SERVER="ns33.domaincontrol.com:53"

# API v3
make test TEST_ZONE_NAME=snowdrop.dev. TEST_DNS_SERVER="ns33.domaincontrol.com:53" TEST_API_VERSION=v3

# With extended timeout (useful when DNS propagation is slow)
make test TEST_ZONE_NAME=snowdrop.dev. TEST_DNS_SERVER="ns33.domaincontrol.com:53" TEST_API_VERSION=v3 TEST_TIMEOUT=5m
```

### Unit tests (no credentials needed)

You can also run just the unit tests, which use mock HTTP servers and do not require GoDaddy credentials or a cluster:

```fish
make test-unit    # all internal package tests
make test-v1      # v1 client tests only
make test-v3      # v3 client tests only
```

### Troubleshooting

- **DNS SERVFAIL or REFUSED**: use the authoritative nameserver for your domain instead of a public resolver.
- **Timeout waiting for propagation**: increase `TEST_TIMEOUT` (e.g., `5m` or `10m`).
- **DUPLICATE_RECORD errors**: a previous test run may have left a stale record. Delete `cert-manager-dns01-tests` TXT records from your domain's DNS zone manually via the GoDaddy dashboard, then retry.
- **Namespace finalizer errors**: wait a moment and retry -- the envtest kube-apiserver may still be cleaning up from a previous run.