module github.com/snowdrop/godaddy-webhook

go 1.12

require (
	github.com/jetstack/cert-manager v0.12.0
	k8s.io/apiextensions-apiserver v0.0.0-20191114105449-027877536833
	k8s.io/client-go v11.0.1-0.20190409021438-1a26190bd76a+incompatible
)

replace k8s.io/client-go => k8s.io/client-go v0.0.0-20190413052642-108c485f896e
