// Copyright 2019 The Kubernetes Authors.
// SPDX-License-Identifier: Apache-2.0

package main_test

import (
	"testing"

	kusttest_test "sigs.k8s.io/kustomize/api/testutils/kusttest"
)

func Test(t *testing.T) {
	th := kusttest_test.MakeEnhancedHarness(t)
	defer th.Reset()

	m := th.LoadAndRunGenerator(`
apiVersion: imranismail.dev/v1
kind: ExternalSecret
type: Opaque
metadata:
  name: my-secret
spec:
  secretsManagerConfig:
    region: "ap-southeast-1"
  dataFrom:
  - secretsManagerRef:
      name: "myapp/production"
  - secretsManagerRef:
      name: "myapp/production"
      region: "ap-northeast-1"
  data:
  - key: "DB_HOSTNAME"
    value: "some-custom-hostname"
  - key: "DB_PASSWORD"
    valueFrom:
      secretsManagerRef:
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
