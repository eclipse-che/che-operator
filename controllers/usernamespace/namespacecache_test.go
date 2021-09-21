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

package usernamespace

import (
	"context"
	"sync"
	"testing"

	dwo "github.com/devfile/devworkspace-operator/apis/controller/v1alpha1"
	"github.com/devfile/devworkspace-operator/pkg/infrastructure"
	v1 "github.com/eclipse-che/che-operator/api/v1"

	projectv1 "github.com/openshift/api/project/v1"
	routev1 "github.com/openshift/api/route/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	extensions "k8s.io/api/extensions/v1beta1"
	"k8s.io/api/node/v1alpha1"
	rbac "k8s.io/api/rbac/v1"

	configv1 "github.com/openshift/api/config/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func createTestScheme() *runtime.Scheme {
	scheme := runtime.NewScheme()
	utilruntime.Must(v1alpha1.AddToScheme(scheme))
	utilruntime.Must(extensions.AddToScheme(scheme))
	utilruntime.Must(corev1.AddToScheme(scheme))
	utilruntime.Must(appsv1.AddToScheme(scheme))
	utilruntime.Must(rbac.AddToScheme(scheme))
	utilruntime.Must(routev1.AddToScheme(scheme))
	utilruntime.Must(v1.AddToScheme(scheme))
	utilruntime.Must(dwo.AddToScheme(scheme))
	utilruntime.Must(projectv1.AddToScheme(scheme))
	utilruntime.Must(configv1.AddToScheme(scheme))

	return scheme
}

func TestGetNamespaceInfoReadsFromCache(t *testing.T) {
	test := func(infraType infrastructure.Type, namespace metav1.Object) {
		infrastructure.InitializeForTesting(infraType)
		ctx := context.TODO()

		ns := namespace.GetName()
		cl := fake.NewFakeClientWithScheme(createTestScheme(), namespace.(runtime.Object))

		nsc := namespaceCache{
			client:          cl,
			knownNamespaces: map[string]namespaceInfo{},
			lock:            sync.Mutex{},
		}

		_, err := nsc.GetNamespaceInfo(ctx, ns)
		if err != nil {
			t.Fatal(err)
		}

		if _, ok := nsc.knownNamespaces[ns]; !ok {
			t.Fatal("The namespace info should have been cached")
		}
	}

	test(infrastructure.Kubernetes, &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "ns",
		},
	})

	test(infrastructure.OpenShiftv4, &projectv1.Project{
		ObjectMeta: metav1.ObjectMeta{
			Name: "prj",
		},
	})
}

func TestExamineUpdatesCache(t *testing.T) {
	test := func(infraType infrastructure.Type, namespace metav1.Object) {
		ctx := context.TODO()

		nsName := namespace.GetName()
		cl := fake.NewFakeClientWithScheme(createTestScheme(), namespace.(runtime.Object))
		infrastructure.InitializeForTesting(infraType)

		nsc := namespaceCache{
			client:          cl,
			knownNamespaces: map[string]namespaceInfo{},
			lock:            sync.Mutex{},
		}

		nsi, err := nsc.GetNamespaceInfo(ctx, nsName)
		if err != nil {
			t.Fatal(err)
		}

		if nsi.OwnerUid != "" {
			t.Fatalf("Detected owner UID should be empty but was %s", nsi.OwnerUid)
		}

		if _, ok := nsc.knownNamespaces[nsName]; !ok {
			t.Fatal("The namespace info should have been cached")
		}

		ns := namespace.(runtime.Object).DeepCopyObject()
		if err := cl.Get(ctx, client.ObjectKey{Name: nsName}, ns); err != nil {
			t.Fatal(err)
		}

		ns.(metav1.Object).SetLabels(map[string]string{
			workspaceNamespaceOwnerUidLabel: "uid",
		})

		if err := cl.Update(ctx, ns); err != nil {
			t.Fatal(err)
		}

		nsi, err = nsc.ExamineNamespace(ctx, nsName)
		if err != nil {
			t.Fatal(err)
		}

		if nsi.OwnerUid != "uid" {
			t.Fatalf("Detected owner UID should be 'uid' but was '%s'", nsi.OwnerUid)
		}
	}

	test(infrastructure.Kubernetes, &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "ns",
		},
	})

	test(infrastructure.OpenShiftv4, &projectv1.Project{
		ObjectMeta: metav1.ObjectMeta{
			Name: "prj",
		},
	})
}
