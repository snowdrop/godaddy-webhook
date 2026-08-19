# ACME Webhook for GoDaddy

Table of Contents
=================
- [ACME Webhook for GoDaddy](#acme-webhook-for-godaddy)
- [Table of Contents](#table-of-contents)
  - [Introduction](#introduction)
  - [Governance](#governance)
  - [Platform](#platform)
  - [Installation](#installation)
    - [Cert Manager](#cert-manager)
    - [The GoDaddy webhook](#the-godaddy-webhook)
      - [Helm deployment](#helm-deployment)
      - [Manual installation](#manual-installation)
  - [Issuer](#issuer)
    - [Secret](#secret)
    - [ClusterIssuer](#clusterissuer)
  - [API Version](#api-version)
  - [Development](#development)
    - [Project structure](#project-structure)
    - [Unit tests](#unit-tests)
    - [DNS resolver test](#dns-resolver-test)
      - [Common testing issues](#common-testing-issues)
    - [Generate the container image](#generate-the-container-image)
  - [Release](#release)

## Introduction

This project maintains the code used by the [certificate manager](https://cert-manager.io/docs/configuration/acme/dns01/) to access the GoDaddy [DNS provider](https://www.godaddy.com/) using a Kubernetes webhook
which needs to be deployed on your kubernetes cluster. When called, the webhook will execute an [ACME DNS challenge](https://cert-manager.io/docs/configuration/acme/) request to the DNS provider
to verify if the provider hosts the domain you are requesting a certificate.

This project supports the following versions of the certificate manager:

| Certificate Manager | GoDaddy webhook    |
|---------------------|--------------------|
| [1.6 - 1.12]        | v0.1.0             | 
| [> 1.13]            | [v0.2.0 .. v0.5.0] |
| [1.16.x]            | [v0.6.0 .. v0.7.0] |
| [1.17.4 - LTS]      | v0.8.0             |

**Remark**: The Helm chart `AppVersion` like the image `version` are tagged according to the version used to release this project: v0.1.0, v0.2.0, etc. When using the main branch, the Helm chart will install the latest image pushed on [quay.io](https://quay.io/repository/snowdrop/cert-manager-webhook-godaddy)

## Governance

Before to open a ticket, please review the [Cert Manager documentation](https://cert-manager.io/docs) explaining the different concepts you will have to deal with such: Issuer, Certificate, Challenge, Order, etc

The troubleshooting section of the documentation is also a good place to start to understand how to debug the different issues you could face: https://cert-manager.io/docs/troubleshooting/acme/.

## Platform

The image built supports as Arch: am64 and arm64 since release `>= 0.2.0`

## Installation

### Cert Manager

Follow the [instructions](https://cert-manager.io/docs/installation/) using the cert manager documentation to install it within your cluster.
On kubernetes (>= 1.21), the process is pretty straightforward if you use the following commands:
```bash
kubectl apply -f https://github.com/jetstack/cert-manager/releases/download/v1.17.4/cert-manager.yaml
```
**NOTES**: Check the cert-manager releases note to verify which [version of certmanager](https://cert-manager.io/docs/installation/supported-releases/) is supported with Kubernetes or OpenShift

### The GoDaddy webhook

#### Helm deployment

When the cert-manager has been installed, deploy the helm chart on your machine using this command:
```bash
export DOMAIN=acme.mydomain.com  # replace with your domain
helm install -n cert-manager godaddy-webhook ./deploy/charts/godaddy-webhook --set groupName=$DOMAIN
```

The `groupName` refers to a prior nonexistent Kubernetes API Group, under which custom resources are created.
The name itself has no connection to the domain names for which certificates are issued, and using the default of
`acme.mycompany.com` is fine.

**NOTE**: The kubernetes resources used to install the Webhook should be deployed within the same namespace as the cert-manager.

- To change one of the values, create a `my-values.yml` file or set the value(s) using helm's `--set` argument:
```bash
helm install -n cert-manager godaddy-webhook -f my-values.yml ./deploy/charts/godaddy-webhook

or

helm install -n cert-manager godaddy-webhook --set pod.securePort=8443 ./deploy/charts/godaddy-webhook
```

You can also use the Helm chart published on gh-pages
```bash
export DOMAIN=acme.mydomain.com  # replace with your domain
helm repo add godaddy-webhook https://snowdrop.github.io/godaddy-webhook
helm install acme-webhook godaddy-webhook/godaddy-webhook -n cert-manager --set groupName=$DOMAIN
```

To uninstall the webhook:
```bash
helm uninstall acme-webhook -n cert-manager
```

#### Manual installation

Alternatively, you can install the webhook using the kubernetes YAML resources. The namespace
  where the resources should be installed is: `cert-manager`
```bash
export DOMAIN=acme.mydomain.com  # replace with your domain
sed "s/acme.mycompany.com/$DOMAIN/g" deploy/webhook-all.yml | kubectl apply --validate=false -f -
```

## Issuer

In order to communicate with GoDaddy DNS provider, we will create a Kubernetes Secret
to store the GoDaddy `API` and `GoDaddy Secret`. 
Next, we will define a `ClusterIssuer` containing the information to access the ACME Letsencrypt Server
and the DNS provider to be used

### Secret

Create a `Secret` containing your GoDaddy credentials. The format depends on the API version you use:

- **API v1** — concatenation of the GoDaddy API key and secret separated by `:`
- **API v3** — a Personal Access Token (see [Creating a PAT](#creating-a-personal-access-token-pat))

```yaml
cat <<EOF > secret.yml
apiVersion: v1
kind: Secret
metadata:
  name: godaddy-api-key
type: Opaque
stringData:
  # For API v1: <GODADDY_API_KEY>:<GODADDY_SECRET_KEY>
  # For API v3: <YOUR_PERSONAL_ACCESS_TOKEN>
  token: <YOUR_CREDENTIALS>
EOF
```
- Next, deploy it under the namespace where you would like to get your certificate/key signed by the ACME CA Authority (e.g. cert-manager)
```bash
kubectl apply -f secret.yml -n <NAMESPACE>
```

### ClusterIssuer

- Create a `ClusterIssuer` resource to specify the address of the ACME staging or production server to access.
  Add the DNS01 Solver Config that this webhook will use to communicate with the API of the GoDaddy Server in order to create
   or delete an ACME Challenge TXT record that the DNS Provider will accept/refuse if the domain name exists.

```yaml
cat <<EOF > clusterissuer.yml 
apiVersion: cert-manager.io/v1
kind: ClusterIssuer
metadata:
  name: letsencrypt-prod
spec:
  acme:
    # ACME Server
    # prod : https://acme-v02.api.letsencrypt.org/directory
    # staging : https://acme-staging-v02.api.letsencrypt.org/directory
    server: <URL_ACME_SERVER> 
    # ACME Email address
    email: <ACME_EMAIL>
    privateKeySecretRef:
      name: letsencrypt-<ENV> # staging or production
    solvers:
    - selector:
        dnsZones:
        - 'example.com'
      dns01:
        webhook:
          config:
            apiKeySecretRef:
              name: godaddy-api-key
              key: token
            production: true
            # apiVersion: "v3"  # Uncomment to use GoDaddy API v3 (requires a PAT, see API Version section)
            ttl: 600
          groupName: acme.mycompany.com
          solverName: godaddy
EOF
```

> **Note**: By default, the webhook uses GoDaddy API **v1**. To use **v3**, add `apiVersion: "v3"` to the webhook config and use a [Personal Access Token](#creating-a-personal-access-token-pat) in your secret instead of an API key pair. See [API Version](#api-version) for details.

- Next, install it on your kubernetes cluster
```bash
kubectl apply -f clusterissuer.yml
```
- Next, create for each of your domain where you need a signed certificate from the Letsencrypt authority the following certificate

```yaml
cat <<EOF > certificate.yml
apiVersion: cert-manager.io/v1
kind: Certificate
metadata:
  name: wildcard-example-com
spec:
  secretName: wildcard-example-com-tls
  renewBefore: 240h
  dnsNames:
  - '*.example.com'
  issuerRef:
    name: letsencrypt-prod
    kind: ClusterIssuer
EOF
```

- Deploy it
```bash
kubectl apply -f certificate.yml -n <NAMESPACE>
```

- If you have deployed a NGinx Ingress Controller on Kubernetes in order to route the trafic to your service
  and to manage the TLS termination, then deploy the following ingress resource where 

```yaml
apiVersion: extensions/v1beta1
kind: Ingress
metadata:
  name: example-ingress
  annotations:
    kubernetes.io/ingress.class: "nginx"
spec:
  tls:
  - hosts:
    - '*.example.com'
    secretName: wildcard-example-com-tls
  rules:
  - host: demo.example.com
    http:
      paths:
      - path: /
        backend:
          serviceName: backend-service
          servicePort: 80
```

- Deploy it
```bash
kubectl apply -f ingress.yml -n <NAMESPACE>
```

**NOTE**: If you prefer to delegate to the certmanager the responsibility to create the Certificate resource, then add the following annotation as described within the documentation `    certmanager.k8s.io/cluster-issuer: "letsencrypt-prod"`

## API Version

This webhook supports two versions of the GoDaddy API. You can select which version to use via the `apiVersion` field in the webhook solver config.

> **Deprecation notice**: GoDaddy will deprecate API v1 in a future release. Starting with godaddy-webhook **1.x**, API v3 will become the default and v1 support will be removed. We recommend migrating your configuration to `apiVersion: "v3"` now.

### API v1 (default)

The original GoDaddy Domains API. Uses `sso-key` authentication with an API key and secret pair.

```yaml
dns01:
  webhook:
    config:
      apiKeySecretRef:
        name: godaddy-api-key
        key: token
      production: true
      apiVersion: "v1"
      ttl: 600
    groupName: acme.mycompany.com
    solverName: godaddy
```

When `apiVersion` is omitted, it defaults to `"v1"`.

### API v3

The newer GoDaddy Domains API. This version introduces several breaking changes compared to v1.

#### What changes with v3

| | API v1 | API v3 |
|---|--------|--------|
| **Authentication** | `sso-key` (API key + secret) | **Bearer token** (Personal Access Token) |
| **Endpoint paths** | `/v1/domains/{domain}/records/TXT/{name}` | `/v3/domains/zones/{zone}/dns-records` |
| **Record filtering** | Path segments | Query parameters (`?type=TXT&name=...`) |
| **Create method** | `PUT` (replace all) | `POST` (append) |
| **Delete method** | `DELETE` by type/name | `DELETE` by record ID (auto-resolved) |
| **Secret format** | `<API_KEY>:<API_SECRET>` | `<PERSONAL_ACCESS_TOKEN>` |

See the [GoDaddy v3 API documentation](https://developer.godaddy.com/en/docs/references/rest/domains/v3/records) for details.

#### Creating a Personal Access Token (PAT)

API v3 **does not support** the legacy `sso-key` authentication. You must create a Personal Access Token:

1. Go to the [GoDaddy Developer Portal](https://developer.godaddy.com)
2. Navigate to **API Keys** > **Create New API Key**
3. Select **Personal Access Token** as the key type
4. Grant the following scopes:
   - `domains.domain:read`
   - `domains.dns:update`
5. Copy the generated token

#### Secret for v3

The Kubernetes secret for v3 contains the PAT as a single value (no colon-separated key pair):

```yaml
cat <<EOF > secret.yml
apiVersion: v1
kind: Secret
metadata:
  name: godaddy-api-key
type: Opaque
stringData:
  token: <YOUR_PERSONAL_ACCESS_TOKEN>
EOF
```

```bash
kubectl apply -f secret.yml -n <NAMESPACE>
```

#### ClusterIssuer config for v3

```yaml
dns01:
  webhook:
    config:
      apiKeySecretRef:
        name: godaddy-api-key
        key: token
      production: true
      apiVersion: "v3"
      ttl: 600
    groupName: acme.mycompany.com
    solverName: godaddy
```

## Development

### Project structure

```
main.go                          -- webhook solver (entry point)
dns_resolver_test.go             -- cert-manager conformance tests
internal/
  auth/auth.go                   -- credential extraction from K8s secrets
  auth/auth_test.go              -- auth unit tests
  dns/dns.go                     -- DNS zone and record name helpers
  godaddy/
    types.go                     -- Client interface, DNSRecord, shared HTTP helper
    v1/client.go                 -- GoDaddy API v1 implementation
    v1/client_test.go            -- v1 unit tests
    v3/client.go                 -- GoDaddy API v3 implementation
    v3/client_test.go            -- v3 unit tests
  logging/logging.go             -- log configuration
```

### Unit tests

Run the API client unit tests (no cluster or credentials required):

```bash
make test-unit    # all unit tests
make test-v1      # v1 client tests only
make test-v3      # v3 client tests only
```

### DNS resolver test

The DNS resolver test (`dns_resolver_test.go`) runs the cert-manager conformance suite against a real GoDaddy domain. It creates and deletes actual TXT records, so use it sparingly.

**IMPORTANT**: Do not run this test too frequently — GoDaddy DNS servers may fail and report `SERVFAIL`.

#### Setup

Create secret files with your GoDaddy credentials. The example template is at `testdata/godaddy/godaddy.secret.example` and must be generated into each API version folder:

```bash
# For API v1 (API key + secret separated by ':')
export GODADDY_TOKEN=$(echo -n "<GODADDY_API_KEY>:<GODADDY_SECRET_KEY>")
envsubst < testdata/godaddy/godaddy.secret.example > testdata/godaddy/v1/secret.yaml

# For API v3 (Personal Access Token)
export GODADDY_TOKEN=$(echo -n "<YOUR_PERSONAL_ACCESS_TOKEN>")
envsubst < testdata/godaddy/godaddy.secret.example > testdata/godaddy/v3/secret.yaml
```

Install the kubebuilder test binaries (etcd, kube-apiserver, kubectl):

```bash
./scripts/fetch-test-binaries.sh
```

#### Running with API v1 (default)

```bash
make test TEST_ZONE_NAME=<YOUR_DOMAIN>. TEST_DNS_SERVER="<NAMESERVER>:53"
```

#### Running with API v3

```bash
make test TEST_ZONE_NAME=<YOUR_DOMAIN>. TEST_DNS_SERVER="<NAMESERVER>:53" TEST_API_VERSION=v3
```

#### Increasing the propagation timeout

By default, the test waits up to **3 minutes** for DNS propagation. If GoDaddy DNS is slow, you can increase this via `TEST_TIMEOUT`:

```bash
make test TEST_ZONE_NAME=<YOUR_DOMAIN>. TEST_DNS_SERVER="<NAMESERVER>:53" TEST_TIMEOUT=5m
```

#### Example

```bash
# Find your domain's authoritative nameserver
dig NS snowdrop.dev +short
# ns33.domaincontrol.com.

# Run with v1
make test TEST_ZONE_NAME=snowdrop.dev. TEST_DNS_SERVER="ns33.domaincontrol.com:53"

# Run with v3
make test TEST_ZONE_NAME=snowdrop.dev. TEST_DNS_SERVER="ns33.domaincontrol.com:53" TEST_API_VERSION=v3
```

#### Common testing issues

- **`SERVFAIL` or `REFUSED` during DNS propagation check**: The integration test creates a real TXT record via the GoDaddy API and then queries a DNS server to verify the record propagated. Public resolvers like `1.1.1.1` may return `SERVFAIL` due to slow propagation. To work around this, use the authoritative nameserver for your domain. Find it with:
  ```bash
  dig NS <YOUR_DOMAIN> +short
  ```
  Then pass it to the test:
  ```bash
  make test TEST_ZONE_NAME=<YOUR_DOMAIN>. TEST_DNS_SERVER="<NAMESERVER>:53"
  # Example: make test TEST_ZONE_NAME=snowdrop.dev. TEST_DNS_SERVER="ns33.domaincontrol.com:53"
  ```
- **Slow GoDaddy responses**: If the above does not help, increase the propagation timeout by passing `TEST_TIMEOUT` (default `3m`), e.g. `make test ... TEST_TIMEOUT=5m`.
- **Namespace finalizer errors**: The test could also fail if the kube-apiserver is still finalizing the deletion of a namespace from a previous run (`"status":{"phase":"Terminating"}`). Wait a moment and retry.

### Generate the container image

- Verify first that you have access to a docker server running on your kubernetes or openshift cluster ;-)
- Compile the project locally (to check if no go error are reported)
```bash
make compile
```
- Next, build the container image using the Dockerfile included within this project
```bash
IMAGE_REPOSITORY="quay.io/snowdrop"
docker build -t ${IMAGE_REPOSITORY}/cert-manager-webhook-godaddy .
```
**NOTE**: Change the `IMAGE_REPOSITORY` to point to your container repository where you have access

You can also use the `Makefile` to build/push the container image and pass as parameters the `IMAGE_NAME` and `IMAGE_TAG`. Without `IMAGE_TAG` defined,
docker will tag/push as `latest`

```bash
IMAGE_REPOSITORY="quay.io/snowdrop"
make build IMAGE_NAME=${IMAGE_REPOSITORY}
make push
```

## Release

Releases are driven by `.github/project.yml`. To create a new release:

1. Update `current-version` and `next-version` in `.github/project.yml`:
   ```yaml
   current-version: "0.8.0"
   next-version: "0.9.0"
   ```
2. Push the change to `main`. The `prepare-release` workflow will automatically:
   - Create a `release/<version>` branch
   - Update the Helm chart and image tag to match the release version
   - Open a Pull Request
3. Review and merge the PR. The `release` workflow will then:
   - Build and push the Docker image to quay.io
   - Publish the Helm chart via chart-releaser
   - Tag the release as `v<version>`
   - Bump the chart files to the next development version
