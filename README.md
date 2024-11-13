# ACME Webhook for GoDaddy

Table of Contents
=================
  * [Introduction](#introduction)
  * [Governance](#governance)
  * [Platform](#platform)
  * [Installation](#installation)
      * [Cert Manager](#cert-manager)
      * [The Godaddy webhook](#the-godaddy-webhook)
        * [Helm deployment](#helm-deployment)
        * [Manual installation](#manual-installation)
  * [Issuer](#issuer)
      * [Secret](#secret)
      * [ClusterIssuer](#clusterissuer)
  * [Development](#development)
      * [Running the test suite](#running-the-test-suite)
      * [Generate the container image](#generate-the-container-image)

## Introduction

This project maintains the code used by the [certificate manager](https://cert-manager.io/docs/configuration/acme/dns01/) to access the Godaddy [DNS provider](https://www.godaddy.com/) using a Kubernetes webhook
which needs to be deployed on your kubernetes cluster. When called, the webhook will execute an ACME DNS challenge request to the DNS provider
to verify if the provider hosts the domain you are requesting a certificate.

This project supports the following versions of the certificate manager:

| Certificate Manager | Godaddy webhook    |
|--------------------|--------------------|
| [1.6 - 1.12]       | v0.1.0             | 
| [> 1.13]           | [v0.2.0 .. v0.5.0] |

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
kubectl apply -f https://github.com/jetstack/cert-manager/releases/download/v1.13.0/cert-manager.yaml
```
**NOTES**: Check the cert-manager releases note to verify which [version of certmanager](https://cert-manager.io/docs/installation/supported-releases/) is supported with Kubernetes or OpenShift

### The Godaddy webhook

#### Helm deployment

When the cert-manager has been installed, deploy the helm chart on your machine using this command:
```bash
export DOMAIN=acme.mydomain.com  # replace with your domain
helm install -n cert-manager godaddy-webhook ./deploy/charts/godaddy-webhook --set groupName=$DOMAIN
```

The `groupName` refers to a prior nonexistant Kubernetes API Group, under which custom resources are created.
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

In order to communicate with Godaddy DNS provider, we will create a Kubernetes Secret
to store the Godaddy `API` and `GoDaddy Secret`. 
Next, we will define a `ClusterIssuer` containing the information to access the ACME Letsencrypt Server
and the DNS provider to be used

### Secret

- Create a `Secret` containing as key parameter the concatenation of the Godaddy Api and Secret separated by ":"
```yaml
cat <<EOF > secret.yml
apiVersion: v1
kind: Secret
metadata:
  name: godaddy-api-key
type: Opaque
stringData:
  token: <GODADDY_API_KEY:GODADDY_SECRET_KEY>
EOF
```
- Next, deploy it under the namespace where you would like to get your certificate/key signed by the ACME CA Authority
```bash
kubectl apply -f secret.yml -n <NAMESPACE>
```

### ClusterIssuer

- Create a `ClusterIssuer` resource to specify the address of the ACME staging or production server to access.
  Add the DNS01 Solver Config that this webhook will use to communicate with the API of the Godaddy Server in order to create
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
            ttl: 600
          groupName: acme.mycompany.com
          solverName: godaddy
EOF
```
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

## Development

### Running the test suite

**IMPORTANT**: Use the testsuite carefully and do not launch it too much times as the DNS servers could fail and report such a message `suite.go:62: error waiting for record to be deleted: unexpected error from DNS server: SERVFAIL`

To test one of your registered domains on godaddy, create a secret.yml file using as [example] file(./testdata/godaddy/godaddy.secret.example)
Replace the `$GODADDY_TOKEN` with your Godaddy API token which corresponds to your `<GODADDY_API_KEY>:<GODADDY_SECRET_KEY>`:

```bash
pushd testdata/godaddy
export GODADDY_TOKEN=$(echo -n "<GODADDY_API_KEY:GODADDY_SECRET_KEY>")
envsubst < godaddy.secret.example > secret.yaml
popd
```

Install a kube-apiserver, etcd locally using the following bash script

```bash
./scripts/fetch-test-binaries.sh
```

Now, execute the test suite and pass as parameter the domain name to be tested

```bash
TEST_ASSET_ETCD=_out/kubebuilder/bin/etcd \
TEST_ASSET_KUBECTL=_out/kubebuilder/bin/kubectl \
TEST_ASSET_KUBE_APISERVER=_out/kubebuilder/bin/kube-apiserver \
TEST_ZONE_NAME=<YOUR_DOMAIN.NAME>. go test -v .
```

or the following `make` command
```bash
make test TEST_ZONE_NAME=<YOUR_DOMAIN.NAME>
```
#### Common testing issues

- As godaddy server could be very slow to reply, it could be needed to increase the TTL defined within the `config.json` file. 
  - If increasing the TTL does not solve the issue, you can also try overriding the DNS server used for testing by setting the `TEST_DNS_SERVER` environment variable to match one of the name servers used by your domain. Ex `TEST_DNS_SERVER="pdns01.domaincontrol.com:53"`
- The test could also fail as the kube api server is currently finalizing the deletion of the namespace `"spec":{"finalizers":["kubernetes"]},"status":{"phase":"Terminating"}}`

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
