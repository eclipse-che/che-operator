//
// Copyright (c) 2019-2025 Red Hat, Inc.
// This program and the accompanying materials are made
// available under the terms of the Eclipse Public License 2.0
// which is available at https://www.eclipse.org/legal/epl-2.0/
//
// SPDX-License-Identifier: EPL-2.0
//
// Contributors:
//   Red Hat, Inc. - initial API and implementation
//

package k8s_client

import (
	"context"
	"reflect"
	"testing"

	testclient "github.com/eclipse-che/che-operator/pkg/common/test/test-client"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	diffs = cmp.Options{
		cmpopts.IgnoreFields(corev1.ConfigMap{}, "TypeMeta"),
		cmp.Comparer(func(x, y metav1.ObjectMeta) bool {
			return reflect.DeepEqual(x.Labels, y.Labels)
		}),
	}
)

func TestSync(t *testing.T) {
	fakeClient, _, scheme := testclient.GetTestClients()
	cli := NewK8sClient(fakeClient, scheme)

	cm := &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ConfigMap",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: "eclipse-che",
		},
	}

	done, err := cli.Sync(context.TODO(), cm, nil, diffs)

	assert.NoError(t, err)
	assert.True(t, done)

	newCm := &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ConfigMap",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: "eclipse-che",
		},
		Data: map[string]string{
			"key": "value",
		},
	}

	done, err = cli.Sync(context.TODO(), newCm, nil, diffs)

	assert.NoError(t, err)
	assert.False(t, done)

	newCm = &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ConfigMap",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: "eclipse-che",
		},
		Data: map[string]string{
			"key": "value",
		},
	}

	done, err = cli.Sync(context.TODO(), newCm, nil, diffs)

	assert.NoError(t, err)
	assert.True(t, done)
}
