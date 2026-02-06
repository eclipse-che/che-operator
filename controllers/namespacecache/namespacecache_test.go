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

package namespacecache

import (
	"context"
	"sync"
	"testing"

	"github.com/eclipse-che/che-operator/pkg/common/test"

	"github.com/eclipse-che/che-operator/pkg/common/infrastructure"
	"github.com/stretchr/testify/assert"

	projectv1 "github.com/openshift/api/project/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestGetNamespaceInfoReadsFromCache(t *testing.T) {
	test := func(infraType infrastructure.Type, namespace metav1.Object) {
		infrastructure.InitializeForTesting(infraType)
		ns := namespace.GetName()

		ctx := test.NewCtxBuilder().WithObjects(namespace.(client.Object)).Build()
		cl := ctx.ClusterAPI.Client

		nsc := NamespaceCache{
			Client:          cl,
			KnownNamespaces: map[string]NamespaceInfo{},
			Lock:            sync.Mutex{},
		}

		_, err := nsc.GetNamespaceInfo(context.TODO(), ns)
		assert.NoError(t, err)
		assert.Contains(t, nsc.KnownNamespaces, ns, "The namespace info should have been cached")
	}

	test(infrastructure.Kubernetes, &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "ns",
		},
	})

	test(infrastructure.OpenShiftV4, &projectv1.Project{
		ObjectMeta: metav1.ObjectMeta{
			Name: "prj",
		},
	})
}

func TestExamineUpdatesCache(t *testing.T) {
	test := func(infraType infrastructure.Type, namespace metav1.Object) {
		nsName := namespace.GetName()
		ctx := test.NewCtxBuilder().WithObjects(namespace.(client.Object)).Build()
		cl := ctx.ClusterAPI.Client
		infrastructure.InitializeForTesting(infraType)

		nsc := NamespaceCache{
			Client:          cl,
			KnownNamespaces: map[string]NamespaceInfo{},
			Lock:            sync.Mutex{},
		}

		nsi, err := nsc.GetNamespaceInfo(context.TODO(), nsName)
		assert.NoError(t, err)

		assert.False(t, nsi.IsWorkspaceNamespace, "The namespace should not be found as managed")

		assert.Contains(t, nsc.KnownNamespaces, nsName, "The namespace info should have been cached")

		ns := namespace.(client.Object)
		assert.NoError(t, cl.Get(context.TODO(), client.ObjectKey{Name: nsName}, ns))

		ns.(metav1.Object).SetLabels(map[string]string{
			WorkspaceNamespaceOwnerUidLabel: "uid",
		})

		assert.NoError(t, cl.Update(context.TODO(), ns))

		nsi, err = nsc.ExamineNamespace(context.TODO(), nsName)
		assert.NoError(t, err)

		assert.True(t, nsi.IsWorkspaceNamespace, "namespace should be found as managed using the legacy user UID label")

		ns.(metav1.Object).SetLabels(map[string]string{
			ChePartOfLabel:    ChePartOfLabelValue,
			CheComponentLabel: CheComponentLabelValue,
		})

		assert.NoError(t, cl.Update(context.TODO(), ns))

		nsi, err = nsc.ExamineNamespace(context.TODO(), nsName)
		assert.NoError(t, err)

		assert.True(t, nsi.IsWorkspaceNamespace, "namespace should be found as managed using the part-of and component labels")
	}

	test(infrastructure.Kubernetes, &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "ns",
		},
	})

	test(infrastructure.OpenShiftV4, &projectv1.Project{
		ObjectMeta: metav1.ObjectMeta{
			Name: "prj",
		},
	})
}
