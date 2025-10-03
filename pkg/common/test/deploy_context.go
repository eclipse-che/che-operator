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

package test

import (
	"strings"

	chev2 "github.com/eclipse-che/che-operator/api/v2"
	"github.com/eclipse-che/che-operator/pkg/common/chetypes"
	k8s_client "github.com/eclipse-che/che-operator/pkg/common/k8s-client"
	testclient "github.com/eclipse-che/che-operator/pkg/common/test/test-client"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type DeployContextBuild struct {
	cheCluster *chev2.CheCluster
	initObject []client.Object
}

func NewCtxBuilder() *DeployContextBuild {
	return &DeployContextBuild{
		initObject: []client.Object{},
		cheCluster: getDefaultCheCluster(),
	}
}

func (f *DeployContextBuild) WithObjects(initObjs ...client.Object) *DeployContextBuild {
	f.initObject = append(f.initObject, initObjs...)
	return f
}

func (f *DeployContextBuild) WithCheCluster(cheCluster *chev2.CheCluster) *DeployContextBuild {
	f.cheCluster = cheCluster
	if f.cheCluster != nil && f.cheCluster.TypeMeta.Kind == "" {
		f.cheCluster.TypeMeta = metav1.TypeMeta{
			Kind:       "CheCluster",
			APIVersion: chev2.GroupVersion.String(),
		}
	}
	return f
}

func (f *DeployContextBuild) Build() *chetypes.DeployContext {
	if f.cheCluster != nil {
		f.initObject = append(f.initObject, f.cheCluster)
	}

	fakeClient, discoveryClient, scheme := testclient.GetTestClients(f.initObject...)

	ctx := &chetypes.DeployContext{
		CheCluster: f.cheCluster,
		ClusterAPI: chetypes.ClusterAPI{
			Client:                  fakeClient,
			NonCachingClient:        fakeClient,
			Scheme:                  scheme,
			DiscoveryClient:         discoveryClient,
			ClientWrapper:           k8s_client.NewK8sClient(fakeClient, scheme),
			NonCachingClientWrapper: k8s_client.NewK8sClient(fakeClient, scheme),
		},
		Proxy: &chetypes.Proxy{},
	}

	if f.cheCluster != nil {
		ctx.CheHost = strings.TrimPrefix(f.cheCluster.Status.CheURL, "https://")
	}

	return ctx
}

func getDefaultCheCluster() *chev2.CheCluster {
	return &chev2.CheCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "eclipse-che",
			Namespace: "eclipse-che",
		},
		TypeMeta: metav1.TypeMeta{
			Kind:       "CheCluster",
			APIVersion: chev2.GroupVersion.String(),
		},
		Status: chev2.CheClusterStatus{
			CheURL: "https://che-host",
		},
	}
}
