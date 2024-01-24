//
// Copyright (c) 2019-2023 Red Hat, Inc.
// This program and the accompanying materials are made
// available under the terms of the Eclipse Public License 2.0
// which is available at https://www.eclipse.org/legal/epl-2.0/
//
// SPDX-License-Identifier: EPL-2.0
//
// Contributors:
//   Red Hat, Inc. - initial API and implementation
//

package deploy

import (
	chev2 "github.com/eclipse-che/che-operator/api/v2"
	"github.com/eclipse-che/che-operator/pkg/common/chetypes"
	"github.com/eclipse-che/che-operator/pkg/common/test"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"testing"
)

func TestReload(t *testing.T) {
	chev2.SchemeBuilder.AddToScheme(scheme.Scheme)
	cli := fake.NewFakeClientWithScheme(
		scheme.Scheme,
		&chev2.CheCluster{
			ObjectMeta: metav1.ObjectMeta{
				Namespace:       "eclipse-che",
				Name:            "eclipse-che",
				ResourceVersion: "1",
			},
		})

	ctx := &chetypes.DeployContext{
		CheCluster: &chev2.CheCluster{
			ObjectMeta: metav1.ObjectMeta{
				Namespace:       "eclipse-che",
				Name:            "eclipse-che",
				ResourceVersion: "2",
			},
			Spec: chev2.CheClusterSpec{
				Components: chev2.CheClusterComponents{
					PluginRegistry: chev2.PluginRegistry{
						OpenVSXURL: pointer.StringPtr("https://open-vsx.org"),
					},
				},
			},
		},
		ClusterAPI: chetypes.ClusterAPI{
			Client:           cli,
			NonCachingClient: cli,
			Scheme:           scheme.Scheme,
		},
	}

	err := ReloadCheClusterCR(ctx)
	if err != nil {
		t.Errorf("Failed to reload checluster, %v", err)
	}

	assert.Equal(t, "1", ctx.CheCluster.ObjectMeta.ResourceVersion)
	assert.Nil(t, ctx.CheCluster.Spec.Components.PluginRegistry.OpenVSXURL)
}

func TestFindCheCRinNamespace(t *testing.T) {
	type testCase struct {
		checluster *chev2.CheCluster
		name       string
		namespace  string
		found      bool
	}

	testCases := []testCase{
		{
			name:       "case #1",
			checluster: &chev2.CheCluster{ObjectMeta: metav1.ObjectMeta{Name: "eclipse-che", Namespace: "eclipse-che"}},
			namespace:  "eclipse-che",
			found:      true,
		},
		{
			name:       "case #2",
			checluster: &chev2.CheCluster{ObjectMeta: metav1.ObjectMeta{Name: "eclipse-che", Namespace: "default"}},
			namespace:  "eclipse-che",
			found:      false,
		},
		{
			name:       "case #3",
			checluster: &chev2.CheCluster{ObjectMeta: metav1.ObjectMeta{Name: "eclipse-che", Namespace: "eclipse-che"}},
			namespace:  "",
			found:      true,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			deployContext := test.GetDeployContext(testCase.checluster, []runtime.Object{})
			checluster, err := FindCheClusterCRInNamespace(deployContext.ClusterAPI.Client, testCase.namespace)
			if testCase.found {
				assert.NoError(t, err)
				assert.Equal(t, testCase.checluster.Name, checluster.Name)
			} else {
				assert.Nil(t, checluster)
			}
		})
	}
}
