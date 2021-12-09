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
	"os"

	"github.com/eclipse-che/che-operator/pkg/deploy"
	devworkspace "github.com/eclipse-che/che-operator/pkg/deploy/dev-workspace"
	"github.com/eclipse-che/che-operator/pkg/util"
	"github.com/stretchr/testify/assert"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	orgv1 "github.com/eclipse-che/che-operator/api/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/pointer"

	"testing"
)

func TestEnsureServerExposureStrategy(t *testing.T) {
	type testCase struct {
		name                string
		expectedCr          *orgv1.CheCluster
		devWorkspaceEnabled bool
		initObjects         []runtime.Object
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
			devWorkspaceEnabled: true,
		},
		{
			name: "Multi Host should be enabled if devWorkspace is not enabled",
			expectedCr: &orgv1.CheCluster{
				Spec: orgv1.CheClusterSpec{
					Server: orgv1.CheClusterSpecServer{
						ServerExposureStrategy: "multi-host",
					},
				},
			},
			devWorkspaceEnabled: false,
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

			checluster.Spec.DevWorkspace.Enable = testCase.devWorkspaceEnabled
			ctx := deploy.GetTestDeployContext(checluster, []runtime.Object{})

			defaults := NewDefaultValuesReconciler()
			_, done, err := defaults.Reconcile(ctx)
			assert.True(t, done)
			assert.Nil(t, err)
			assert.Equal(t, testCase.expectedCr.Spec.Server.ServerExposureStrategy, ctx.CheCluster.Spec.Server.ServerExposureStrategy)
		})
	}
}

func TestNativeUserModeEnabled(t *testing.T) {
	type testCase struct {
		name                    string
		initObjects             []runtime.Object
		isOpenshift             bool
		devworkspaceEnabled     bool
		initialNativeUserValue  *bool
		expectedNativeUserValue *bool
	}

	testCases := []testCase{
		{
			name:                    "che-operator should use nativeUserMode when devworkspaces on openshift and no initial value in CR for nativeUserMode",
			isOpenshift:             true,
			devworkspaceEnabled:     true,
			initialNativeUserValue:  nil,
			expectedNativeUserValue: pointer.BoolPtr(true),
		},
		{
			name:                    "che-operator should use nativeUserMode value from initial CR",
			isOpenshift:             true,
			devworkspaceEnabled:     true,
			initialNativeUserValue:  pointer.BoolPtr(false),
			expectedNativeUserValue: pointer.BoolPtr(false),
		},
		{
			name:                    "che-operator should use nativeUserMode value from initial CR",
			isOpenshift:             true,
			devworkspaceEnabled:     true,
			initialNativeUserValue:  pointer.BoolPtr(true),
			expectedNativeUserValue: pointer.BoolPtr(true),
		},
		{
			name:                    "che-operator should use nativeUserMode when devworkspaces on kubernetes and no initial value in CR for nativeUserMode",
			isOpenshift:             false,
			devworkspaceEnabled:     true,
			initialNativeUserValue:  nil,
			expectedNativeUserValue: pointer.BoolPtr(true),
		},
		{
			name:                    "che-operator not modify nativeUserMode when devworkspace not enabled",
			isOpenshift:             true,
			devworkspaceEnabled:     false,
			initialNativeUserValue:  nil,
			expectedNativeUserValue: nil,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			logf.SetLogger(zap.New(zap.WriteTo(os.Stdout), zap.UseDevMode(true)))

			checluster := &orgv1.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "eclipse-che",
					Namespace: "eclipse-che",
				},
			}

			// reread templates (workaround after setting IsOpenShift value)
			util.IsOpenShift = testCase.isOpenshift
			devworkspace.DevWorkspaceTemplates = devworkspace.DevWorkspaceTemplatesPath()
			devworkspace.DevWorkspaceIssuerFile = devworkspace.DevWorkspaceTemplates + "/devworkspace-controller-selfsigned-issuer.Issuer.yaml"
			devworkspace.DevWorkspaceCertificateFile = devworkspace.DevWorkspaceTemplates + "/devworkspace-controller-serving-cert.Certificate.yaml"

			checluster.Spec.DevWorkspace.Enable = testCase.devworkspaceEnabled
			checluster.Spec.Auth.NativeUserMode = testCase.initialNativeUserValue
			ctx := deploy.GetTestDeployContext(checluster, []runtime.Object{})

			defaults := NewDefaultValuesReconciler()
			_, done, err := defaults.Reconcile(ctx)
			assert.True(t, done)
			assert.Nil(t, err)
			assert.Equal(t, testCase.expectedNativeUserValue, ctx.CheCluster.Spec.Auth.NativeUserMode)
		})
	}
}
