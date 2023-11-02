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
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
