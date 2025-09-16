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
	"testing"

	testclient "github.com/eclipse-che/che-operator/pkg/common/test/test-client"
	consolev1 "github.com/openshift/api/console/v1"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func TestDeleteClusterScoped(t *testing.T) {
	fakeClient, _, scheme := testclient.GetTestClients(
		&consolev1.ConsoleLink{
			TypeMeta: metav1.TypeMeta{
				Kind:       "ConsoleLink",
				APIVersion: "console.openshift.io/v1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: "test",
			},
		})
	cli := NewK8sClient(fakeClient, scheme)

	done, err := cli.DeleteClusterScoped(context.TODO(), "test", &consolev1.ConsoleLink{})

	assert.NoError(t, err)
	assert.True(t, done)

	err = fakeClient.Get(context.TODO(), types.NamespacedName{Name: "test"}, &consolev1.ConsoleLink{})
	assert.Error(t, err)
	assert.True(t, errors.IsNotFound(err))
}

func TestDeleteClusterScopedNotExistedObject(t *testing.T) {
	fakeClient, _, scheme := testclient.GetTestClients()
	cli := NewK8sClient(fakeClient, scheme)

	done, err := cli.Delete(context.TODO(), types.NamespacedName{Name: "test", Namespace: "eclipse-che"}, &consolev1.ConsoleLink{})

	assert.NoError(t, err)
	assert.True(t, done)

	err = fakeClient.Get(context.TODO(), types.NamespacedName{Name: "test"}, &consolev1.ConsoleLink{})
	assert.Error(t, err)
	assert.True(t, errors.IsNotFound(err))
}
