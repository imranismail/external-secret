## TLDR

Given that you have this deployment manifest

```yaml
# ~/deployments/app/kustomization.yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
generators:
- generator.yaml


# ~/deployments/app/generator.yaml
apiVersion: imranismail.dev/v1
kind: ExternalSecret
# generator options
behavior: create
disableNameSuffixHash: false
type: Opaque
metadata:
  name: my-secret
spec:
  # set aws config to be used to fetch each data
	secretManagerConfig:
		region: "ap-southeast-1"
  dataFrom:
  - secretManagerRef:
      name: "myapp/production"
	- secretManagerRef:
			name: "myapp/production"
      # override .secretManagerConfig.region
			region: "ap-northeast-1"
  data:
  - key: "DB_HOSTNAME"
    value: "some-custom-hostname"
  - key: "DB_PASSWORD"
    valueFrom:
      secretManagerKeyRef:
        name: "myapp/production"
        # look up key in secret
				key: "db-password"
        # override .secretManagerConfig.region
				region: "ap-northeast-1"
  # take the whole secret as a file
  - key: "secret.json"
    valueFrom:
      secretManagerKeyRef:
        name: "myapp/production"
        # omit key to take the whole secret as a file
				# key: "db-password"
```

## AWS Credentials

This tool relies on the default behavior of the AWS SDK for Go to determine AWS credentials and region.