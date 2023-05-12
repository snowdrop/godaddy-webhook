# ACME Webhook for GoDaddy

Table of Contents
=================

  * [Installation](#installation)
      * [Cert Manager](#cert-manager)
      * [The Webhook](#the-webhook)
  * [Issuer](#issuer)
      * [Secret](#secret)
      * [ClusterIssuer](#clusterissuer)
  * [Development](#development)
      * [Running the test suite](#running-the-test-suite)
      * [Generate the container image](#generate-the-container-image)

## Installation

### Cert Manager

Follow the [instructions](https://cert-manager.io/docs/installation/) using the cert manager documentation to install it within your cluster.
On kubernetes (>= 1.21), the process is pretty straightforward if you use the following commands:
```bash
kubectl apply -f https://github.com/cert-manager/cert-manager/releases/download/v1.11.2/cert-manager.yaml
```
**NOTES**: Check the cert-manager releases note to verify which [version of certmanager](https://cert-manager.io/docs/installation/supported-releases/) is supported with Kubernetes or OpenShift
### The Webhook

- Install next the helm chart if [helm v3 is deployed](https://helm.sh/docs/intro/install/) on your machine
```bash
helm install -n cert-manager godaddy-webhook ./deploy/helm
```
**NOTE**: The kubernetes resources used to install the Webhook should be deployed within the same namespace as the cert-manager.

- To change one of the values, create a `my-values.yml` file or set the value(s) using helm's `--set` argument:
```bash
helm install -n cert-manager godaddy-webhook --set pod.securePort=8443 ./deploy/helm
```

- To uninstall the webhook:
```bash
$ helm delete godaddy-webhook -n cert-manager
```

- Alternatively, you can install the webhook using the list of the kubernetes resources. The namespace
  used to install the resources is `cert-manager`
```bash
 kubectl apply -f deploy/webhook-all.yml --validate=false
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
  token: <GODADDY_API:GODADDY_SECRET>
EOF
```
- Next, deploy it under the namespace where you would like to get your certificate/key signed by the ACME CA Authority
```bash
kubectl apply -f secret.yml -n <NAMESPACE>
```

### ClusterIssuer

- Create a `ClusterIssuer`resource to specify the address of the ACME staging or production server to access.
  Add the DNS01 Solver Config that this webhook will use to communicate with the API of the Godaddy Server in order to create
   or delete an ACME Challenge TXT record that the DNS Provider will accept/refuse if the domain name exists.

```yaml
cat <<EOF > clusterissuer.yml 
EOF apiVersion: cert-manager.io/v1
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
        dnsNames:
        - '*.example.com'
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

**IMPORTANT**: Use the tetsuite carefully and do not launch it too much times as the DNS servers could fail and report such a message `suite.go:62: error waiting for record to be deleted: unexpected error from DNS server: SERVFAIL`

To test one of your registered domains on godaddy, create a secret.yml file using as [example] file(./testdata/godaddy/godaddy.secret.example)
Replace the `$GODADDY_TOKEN` with your Godaddy API token which corresponds to your `<GODADDY_KEY>:<GODADDY_SECRET>`:

```bash
pushd testdata/godaddy
export GODADDY_TOKEN=$(echo -n "<GODADDY_KEY:GODADDY_SECRET>")
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
**IMPORTANT**: As godaddy server could be very slow to reply, it could be needed to increase the TTL defined within the `config.json` file. The test could also fail
as the kube api server is currently finalizing the deletion of the namespace `"spec":{"finalizers":["kubernetes"]},"status":{"phase":"Terminating"}}`

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
