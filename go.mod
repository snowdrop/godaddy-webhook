module github.com/snowdrop/godaddy-webhook

go 1.16

require (
	github.com/jetstack/cert-manager v1.6.1
	k8s.io/apiextensions-apiserver v0.22.2
	k8s.io/apimachinery v0.22.2
	k8s.io/client-go v0.22.2
)

replace github.com/prometheus/client_golang => github.com/prometheus/client_golang v1.11.0
