//
// Copyright (c) 2021 Red Hat, Inc.
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
	"os"

	orgv1 "github.com/eclipse/che-operator/pkg/apis/org/v1"
	"github.com/eclipse/che-operator/pkg/util"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"

	"testing"
)

const (
	finalizer = "some.finalizer"
)

func TestAppendFinalizer(t *testing.T) {
	cheCluster := &orgv1.CheCluster{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "eclipse-che",
			Name:      "eclipse-che",
		},
	}
	logf.SetLogger(zap.LoggerTo(os.Stdout, true))
	orgv1.SchemeBuilder.AddToScheme(scheme.Scheme)
	cli := fake.NewFakeClientWithScheme(scheme.Scheme, cheCluster)

	deployContext := &DeployContext{
		CheCluster: cheCluster,
		ClusterAPI: ClusterAPI{
			Client: cli,
			Scheme: scheme.Scheme,
		},
	}

	err := AppendFinalizer(deployContext, finalizer)
	if err != nil {
		t.Fatalf("Failed to append finalizer: %v", err)
	}

	if !util.ContainsString(deployContext.CheCluster.ObjectMeta.Finalizers, finalizer) {
		t.Fatalf("Failed to append finalizer: %v", err)
	}
}

func TestDeleteFinalizer(t *testing.T) {
	cheCluster := &orgv1.CheCluster{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:  "eclipse-che",
			Name:       "eclipse-che",
			Finalizers: []string{finalizer},
		},
	}
	logf.SetLogger(zap.LoggerTo(os.Stdout, true))
	orgv1.SchemeBuilder.AddToScheme(scheme.Scheme)
	cli := fake.NewFakeClientWithScheme(scheme.Scheme, cheCluster)

	deployContext := &DeployContext{
		CheCluster: cheCluster,
		ClusterAPI: ClusterAPI{
			Client: cli,
			Scheme: scheme.Scheme,
		},
	}

	err := DeleteFinalizer(deployContext, finalizer)
	if err != nil {
		t.Fatalf("Failed to append finalizer: %v", err)
	}

	if util.ContainsString(deployContext.CheCluster.ObjectMeta.Finalizers, finalizer) {
		t.Fatalf("Failed to delete finalizer: %v", err)
	}
}
