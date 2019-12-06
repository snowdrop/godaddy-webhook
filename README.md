# ACME Webhook for GoDaddy

## Installation

```bash
$ helm install --name godaddy-webhook --namespace cert-manager ./deploy/godaddy-webhook
```

## Issuer

In order to communicate with Godaddy DNS provider, we will create a Kubernetes Secret
to store your `GoDaddy API` and `GoDaddy Secret`. 
Next, we will define a ClusterIssuer containing the definition of the ACME Server - Letsencrypt
and the DNS provider to be used

### Secret

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: gadaddy-api-key
type: Opaque
stringData:
  key: <GODADDY_API:GODADDY_SECRET>
```

### ClusterIssuer

```yaml
apiVersion: certmanager.k8s.io/v1alpha1
kind: ClusterIssuer
metadata:
  name: letsencrypt-prod
spec:
  acme:
    server: https://acme-v02.api.letsencrypt.org/directory
    email: <your email>
    privateKeySecretRef:
      name: letsencrypt-prod
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
            authApiKey: <your GoDaddy authAPIKey>
            authApiSecret: <your GoDaddy authApiSecret>
            production: true
            ttl: 600
          groupName: acme.mycompany.com
          solverName: godaddy
```

Certificate

```yaml
apiVersion: certmanager.k8s.io/v1alpha1
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
```

Ingress

```yaml
apiVersion: extensions/v1beta1
kind: Ingress
metadata:
  name: example-ingress
  namespace: default
  annotations:
    certmanager.k8s.io/cluster-issuer: "letsencrypt-prod"
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
