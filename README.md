## TLDR

A kustomize exec plugin to generate secret from remote stores. Currently supports AWS SecretsManager

Given that you have this kustomization:

**kustomization.yaml**

```yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
generators:
- secret.yaml
```

**secret.yaml**
```yaml
apiVersion: imranismail.dev/v1
kind: ExternalSecret
# generator options
behavior: create
disableNameSuffixHash: false
type: Opaque
metadata:
  name: my-secret
  annotations:
    whatever: "whatever"
  labels:
    whatever: "whatever"
spec:
  # set aws config to be used to fetch each data
  secretsManagerConfig:
    region: "ap-southeast-1"
  dataFrom:
  - secretsManagerRef:
      name: "myapp/production"
  - secretsManagerRef:
      name: "myapp/production"
      # override .secretsManagerConfig.region
      region: "ap-northeast-1"
  data:
  - key: "DB_HOSTNAME"
    value: "some-custom-hostname"
  - key: "DB_PASSWORD"
    valueFrom:
      secretsManagerRef:
        name: "myapp/production"
        # look up key in secret
        key: "db-password"
        # override .secretsManagerConfig.region
        region: "ap-northeast-1"
  # take the whole secret as a file
  - key: "secret.json"
    valueFrom:
      secretsManagerRef:
        name: "myapp/production"
        # omit key to take the whole secret as a file
        # key: "db-password"
```

It outputs this:

```yaml
apiVersion: imranismail.dev/v1
kind: Secret
metadata:
  name: my-secret
  annotations:
    whatever: "whatever"
  labels:
    whatever: "whatever"
type: Opaque
data:
  # key and base64 encoded values from remote datastores
  {{key}}: {{val}}
```

## Override Logic

Currently `data` always overrides `dataFrom`. This works similar to Kubernetes Container V1 API for the [`env`](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.16/#envvarsource-v1-core) and [`envFrom`](ps://kubernetes.io/docs/reference/generated/kubernetes-api/v1.16/#envfromsource-v1-core) field.

## AWS Credentials

This tool relies on the default behavior of the AWS SDK for Go to determine AWS credentials and region.
