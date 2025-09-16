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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestGetClusterScopedExistedObject(t *testing.T) {
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

	link := &consolev1.ConsoleLink{}
	exists, err := cli.GetClusterScoped(context.TODO(), "test", link)

	assert.NoError(t, err)
	assert.True(t, exists)
	assert.Equal(t, "console.openshift.io/v1", link.APIVersion)
	assert.Equal(t, "ConsoleLink", link.Kind)
}

func TestGetClusterScopedNotExistedObject(t *testing.T) {
	fakeClient, _, scheme := testclient.GetTestClients()
	cli := NewK8sClient(fakeClient, scheme)

	link := &consolev1.ConsoleLink{}
	exists, err := cli.GetClusterScoped(context.TODO(), "test", link)

	assert.NoError(t, err)
	assert.False(t, exists)
}
