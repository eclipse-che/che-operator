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

package deploy

import (
	"testing"

	chev2 "github.com/eclipse-che/che-operator/api/v2"
	"github.com/eclipse-che/che-operator/pkg/common/test"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
)

func TestReload(t *testing.T) {
	ctx := test.NewCtxBuilder().WithCheCluster(&chev2.CheCluster{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:       "eclipse-che",
			Name:            "eclipse-che",
			ResourceVersion: "1",
		},
	}).Build()

	ctx.CheCluster = &chev2.CheCluster{
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
			deployContext := test.NewCtxBuilder().WithCheCluster(testCase.checluster).Build()
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
