// Copyright 2019 The Kubernetes Authors.
// SPDX-License-Identifier: Apache-2.0

package main_test

import (
	"testing"

	kusttest_test "sigs.k8s.io/kustomize/api/testutils/kusttest"
)

func Test(t *testing.T) {
	tc := kusttest_test.NewPluginTestEnv(t).Set()
	defer tc.Reset()

	tc.BuildExecPlugin("imranismail.dev", "v1", "ExternalSecret")

	th := kusttest_test.NewKustTestHarnessAllowPlugins(t, "/app")

	m := th.LoadAndRunGenerator(`
apiVersion: imranismail.dev/v1
kind: ExternalSecret
type: Opaque
metadata:
  name: my-secret
spec:
	secretManagerConfig:
		region: "ap-southeast-1"
  dataFrom:
  - secretManagerRef:
      name: "myapp/production"
	- secretManagerRef:
			name: "myapp/production"
			region: "ap-northeast-1"
  data:
  - key: "DB_HOSTNAME"
    value: "some-custom-hostname"
  - key: "DB_PASSWORD"
    valueFrom:
      secretManagerKeyRef:
        name: "myapp/production"
				key: "db-password"
				region: "ap-northeast-1"
`)

	th.AssertActualEqualsExpected(m, `
apiVersion: v1
data:
kind: Secret
metadata:
	creationTimestamp: null
	name: my-secret
type: Opaque
`)
}
