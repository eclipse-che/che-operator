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
package deploy

import (
	orgv1 "github.com/eclipse-che/che-operator/api/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"testing"
)

func TestReload(t *testing.T) {
	cheCluster := &orgv1.CheCluster{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:       "eclipse-che",
			Name:            "eclipse-che",
			ResourceVersion: "1",
		},
	}

	orgv1.SchemeBuilder.AddToScheme(scheme.Scheme)
	cli := fake.NewFakeClientWithScheme(scheme.Scheme, cheCluster)

	cheCluster = &orgv1.CheCluster{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:       "eclipse-che",
			Name:            "eclipse-che",
			ResourceVersion: "2",
		},
	}

	deployContext := &DeployContext{
		CheCluster: cheCluster,
		ClusterAPI: ClusterAPI{
			Client:           cli,
			NonCachingClient: cli,
			Scheme:           scheme.Scheme,
		},
	}

	err := ReloadCheClusterCR(deployContext)
	if err != nil {
		t.Errorf("Failed to reload checluster, %v", err)
	}

	if cheCluster.ObjectMeta.ResourceVersion != "1" {
		t.Errorf("Failed to reload checluster")
	}
}
