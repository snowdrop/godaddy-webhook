module github.com/snowdrop/godaddy-webhook

go 1.13

require (
	github.com/jetstack/cert-manager v0.12.0
	k8s.io/apiextensions-apiserver v0.0.0-20191114105449-027877536833
	k8s.io/client-go v0.0.0-20191114101535-6c5935290e33
	k8s.io/component-base v0.0.0-20191114102325-35a9586014f7
// k8s.io/client-go v11.0.1-0.20191029005444-8e4128053008+incompatible // Corresponds to k8s 1.14
)

replace github.com/prometheus/client_golang => github.com/prometheus/client_golang v0.9.4

// replace k8s.io/apiextensions-apiserver => k8s.io/apiextensions-apiserver v0.0.0-20190413053546-d0acb7a76918
// replace k8s.io/component-base => k8s.io/component-base v0.0.0-20190413053003-a7e0d79a8811
