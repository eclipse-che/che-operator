//
// Copyright (c) 2019-2021 Red Hat, Inc.
// This program and the accompanying materials are made
// available under the terms of the Eclipse Public License 2.0
// which is available at https://www.eclipse.org/legal/epl-2.0/
//
// SPDX-License-Identifier: EPL-2.0
//
// Contributors:
//   Red Hat, Inc. - initial API and implementation
//
package server

import (
	"github.com/eclipse-che/che-operator/pkg/deploy"
	"github.com/stretchr/testify/assert"

	orgv1 "github.com/eclipse-che/che-operator/api/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"testing"
)

func TestEnsureServerExposureStrategy(t *testing.T) {
	type testCase struct {
		name        string
		expectedCr  *orgv1.CheCluster
		initObjects []runtime.Object
	}

	testCases := []testCase{
		{
			name: "Single Host should be enabled if devWorkspace is enabled",
			expectedCr: &orgv1.CheCluster{
				Spec: orgv1.CheClusterSpec{
					Server: orgv1.CheClusterSpecServer{
						ServerExposureStrategy: "single-host",
					},
				},
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			checluster := &orgv1.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "eclipse-che",
					Namespace: "eclipse-che",
				},
			}

			ctx := deploy.GetTestDeployContext(checluster, []runtime.Object{})

			defaults := NewDefaultValuesReconciler()
			_, done, err := defaults.Reconcile(ctx)
			assert.True(t, done)
			assert.Nil(t, err)
			assert.Equal(t, testCase.expectedCr.Spec.Server.ServerExposureStrategy, ctx.CheCluster.Spec.Server.ServerExposureStrategy)
		})
	}
}
