# ACME Webhook for GoDaddy

## Installation

```bash
$ helm install --name godaddy-webhook --namespace cert-manager ./deploy/godaddy-webhook
```
**NOTE** : The kubernetes resources to install the Webhook should be deployed within the same namespace as the cert-manager

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
  name: gadaddy-api-key
type: Opaque
stringData:
  key: <GODADDY_API:GODADDY_SECRET>
EOF
```
- Next, deploy it under the namespace where you would like to get your certificate/key signed by the ACME CA Authority
```bash
kubectl appy -f secret.yml -n <NAMESPACE>
```

### ClusterIssuer

- Create a `ClusterIssuer`resource to specify the address of the ACME staging or production server to access.
  Add the DNS01 Solver Config that this webhook will use to communicate with the API of the Godaddy Server in order to create
   or delete an ACME Challenge TXT record that the DNS Provider will accept/refuse if the domain name exists.

```yaml
cat <<EOF > clusterissuer.yml 
EOF apiVersion: cert-manager.io/v1alpha2
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
apiVersion: cert-manager.io/v1alpha2
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
  and to manage the TLS termination, then deploy the followiing ingress resource where 

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

**NOTE**: If you prefer to delegate to the certmanager the responsability to create the Certificate resource, then add the following annotation as described within the documentation `    certmanager.k8s.io/cluster-issuer: "letsencrypt-prod"`

## Development

### Running the test suite
All DNS providers **must** run the DNS01 provider conformance testing suite,
else they will have undetermined behaviour when used with cert-manager.

**It is essential that you configure and run the test suite when creating a
DNS01 webhook.**

An example Go test file has been provided in [main_test.go]().

> Prepare

```bash
$ scripts/fetch-test-binaries.sh
```

You can run the test suite using `go`

```bash
$ TEST_ZONE_NAME=example.com. go test .
```

or the following make command
```bash
make test TEST_ZONE_NAME=example.me.
```

The example file has a number of areas you must fill in and replace with your
own options in order for tests to pass.
