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
	"github.com/stretchr/testify/assert"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestList(t *testing.T) {
	fakeClient, _, scheme := testclient.GetTestClients(
		&appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "deployment_1",
				Namespace: "eclipse-che",
			},
			TypeMeta: metav1.TypeMeta{
				Kind:       "Deployment",
				APIVersion: appsv1.SchemeGroupVersion.String(),
			},
		},
		&appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "deployment_2",
				Namespace: "eclipse-che",
			},
			TypeMeta: metav1.TypeMeta{
				Kind:       "Deployment",
				APIVersion: appsv1.SchemeGroupVersion.String(),
			},
		})
	cli := NewK8sClient(fakeClient, scheme)

	objs, err := cli.List(context.TODO(), &appsv1.DeploymentList{})

	assert.NoError(t, err)
	assert.Equal(t, 2, len(objs))

	for _, obj := range objs {
		_, ok := obj.(*appsv1.Deployment)
		assert.Equal(t, "Deployment", obj.GetObjectKind().GroupVersionKind().Kind)
		assert.Equal(t, "v1", obj.GetObjectKind().GroupVersionKind().Version)
		assert.Equal(t, "apps", obj.GetObjectKind().GroupVersionKind().Group)
		assert.True(t, ok)
	}
}
