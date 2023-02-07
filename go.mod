module github.com/imranismail/ExternalSecret

go 1.15

require (
	github.com/aws/aws-sdk-go-v2/config v0.2.1
	github.com/aws/aws-sdk-go-v2/service/secretsmanager v0.28.0
	github.com/go-logr/logr v0.2.1 // indirect
	github.com/google/gofuzz v1.2.0 // indirect
	k8s.io/api v0.20.0
	k8s.io/apimachinery v0.20.0
	k8s.io/client-go v0.20.0 // indirect
	sigs.k8s.io/kustomize/api v0.6.3
	sigs.k8s.io/yaml v1.2.0
)
