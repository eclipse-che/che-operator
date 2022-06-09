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
	"os"

	chev2 "github.com/eclipse-che/che-operator/api/v2"
	"github.com/eclipse-che/che-operator/pkg/common/chetypes"
	"github.com/eclipse-che/che-operator/pkg/common/utils"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"testing"
)

const (
	finalizer = "some.finalizer"
)

func TestAppendFinalizer(t *testing.T) {
	cheCluster := &chev2.CheCluster{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "eclipse-che",
			Name:      "eclipse-che",
		},
	}
	logf.SetLogger(zap.New(zap.WriteTo(os.Stdout), zap.UseDevMode(true)))
	chev2.SchemeBuilder.AddToScheme(scheme.Scheme)
	cli := fake.NewFakeClientWithScheme(scheme.Scheme, cheCluster)

	deployContext := &chetypes.DeployContext{
		CheCluster: cheCluster,
		ClusterAPI: chetypes.ClusterAPI{
			Client: cli,
			Scheme: scheme.Scheme,
		},
	}

	err := AppendFinalizer(deployContext, finalizer)
	if err != nil {
		t.Fatalf("Failed to append finalizer: %v", err)
	}

	if !utils.Contains(deployContext.CheCluster.ObjectMeta.Finalizers, finalizer) {
		t.Fatalf("Failed to append finalizer: %v", err)
	}

	// shouldn't add finalizer twice
	err = AppendFinalizer(deployContext, finalizer)
	if err != nil {
		t.Fatalf("Failed to append finalizer: %v", err)
	}

	if len(deployContext.CheCluster.ObjectMeta.Finalizers) != 1 {
		t.Fatalf("Finalizer shouldn't be added twice")
	}
}

func TestDeleteFinalizer(t *testing.T) {
	cheCluster := &chev2.CheCluster{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:  "eclipse-che",
			Name:       "eclipse-che",
			Finalizers: []string{finalizer},
		},
	}
	logf.SetLogger(zap.New(zap.WriteTo(os.Stdout), zap.UseDevMode(true)))
	chev2.SchemeBuilder.AddToScheme(scheme.Scheme)
	cli := fake.NewFakeClientWithScheme(scheme.Scheme, cheCluster)

	deployContext := &chetypes.DeployContext{
		CheCluster: cheCluster,
		ClusterAPI: chetypes.ClusterAPI{
			Client: cli,
			Scheme: scheme.Scheme,
		},
	}

	err := DeleteFinalizer(deployContext, finalizer)
	if err != nil {
		t.Fatalf("Failed to append finalizer: %v", err)
	}

	if utils.Contains(deployContext.CheCluster.ObjectMeta.Finalizers, finalizer) {
		t.Fatalf("Failed to delete finalizer: %v", err)
	}
}

func TestGetFinalizerNameShouldReturnStringLess64Chars(t *testing.T) {
	expected := "7890123456789012345678901234567891234567.finalizers.che.eclipse"
	prefix := "7890123456789012345678901234567891234567"

	actual := GetFinalizerName(prefix)
	if expected != actual {
		t.Fatalf("Incorrect finalizer name: %s", actual)
	}
}
