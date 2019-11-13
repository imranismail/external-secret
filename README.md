
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
metadata:
  name: forbiddenValues
  namespace: production
disableNameSuffixHash: false
behavior: create
type: Opaque
data:
  hello:
    name: "external-secret-test"
    key: "hello"
```

Compile and install the executable to `~/.config/kustomize/plugins/imranismail/v1/externalsecret/ExternalSecret`

```sh
$ mkdir -p ~/.config/kustomize/plugins/imranismail/v1/externalsecret/ExternalSecret
$ go get github.com/imranismail/kustomize-external-secret
$ ln -s $GOPATH/bin/kustomize-external-secret ~/.config/kustomize/plugins/imranismail/v1/externalsecret/ExternalSecret
```

Build out the kustomization.yaml

```sh
AWS_PROFILE=<aws-profile> kustomize build --enable_alpha_plugins ~/deployments/app
```