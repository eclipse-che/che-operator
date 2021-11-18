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
package devworkspace

import (
	"testing"

	orgv1 "github.com/eclipse-che/che-operator/api/v1"
	"github.com/eclipse-che/che-operator/pkg/deploy"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestShouldSyncNewObject(t *testing.T) {
	deployContext := deploy.GetTestDeployContext(nil, []runtime.Object{})

	newObject := deploy.GetConfigMapSpec(deployContext, "test", map[string]string{}, "test")
	newObject.GetAnnotations()[deploy.CheEclipseOrgHash256] = "hash"

	// tries to sync a new object
	isDone, err := syncObject(deployContext, newObject, "eclipse-che")
	assert.NoError(t, err, "Failed to sync object")
	assert.True(t, isDone, "Failed to sync object")

	// reads object and check content, object is supposed to be created
	actual := &corev1.ConfigMap{}
	exists, err := deploy.GetNamespacedObject(deployContext, "test", actual)
	assert.NoError(t, err, "failed to get configmap")
	assert.True(t, exists, "configmap is not found")

	assert.Equal(t, "eclipse-che", actual.GetAnnotations()[deploy.CheEclipseOrgNamespace], "hash annotation mismatch")
	assert.Equal(t, "hash", actual.GetAnnotations()[deploy.CheEclipseOrgHash256], "namespace annotation mismatch")
}

func TestShouldSyncObjectIfItWasCreatedBySameOriginHashDifferent(t *testing.T) {
	deployContext := deploy.GetTestDeployContext(nil, []runtime.Object{})

	initialObject := deploy.GetConfigMapSpec(deployContext, "test", map[string]string{"a": "b"}, "test")
	initialObject.SetAnnotations(map[string]string{
		deploy.CheEclipseOrgHash256:   "hash",
		deploy.CheEclipseOrgNamespace: "eclipse-che",
	})
	isCreated, err := deploy.Create(deployContext, initialObject)
	assert.NoError(t, err)
	assert.True(t, isCreated)

	// creates initial object
	newObject := deploy.GetConfigMapSpec(deployContext, "test", map[string]string{"a": "c"}, "test")
	newObject.GetAnnotations()[deploy.CheEclipseOrgHash256] = "newHash"

	// tries to sync object with a new
	_, err = syncObject(deployContext, newObject, "eclipse-che")
	assert.NoError(t, err, "Failed to sync object")

	// reads object and check content, object supposed to be updated
	// it was created by the same origin
	actual := &corev1.ConfigMap{}
	exists, err := deploy.GetNamespacedObject(deployContext, "test", actual)
	assert.NoError(t, err, "failed to get configmap")
	assert.True(t, exists, "configmap is not found")

	assert.Equal(t, "eclipse-che", actual.GetAnnotations()[deploy.CheEclipseOrgNamespace], "hash annotation mismatch")
	assert.Equal(t, "newHash", actual.GetAnnotations()[deploy.CheEclipseOrgHash256], "namespace annotation mismatch")
	assert.Equal(t, "c", actual.Data["a"], "data mismatch")
}

func TestShouldNotSyncObjectIfThereIsAnotherCheCluster(t *testing.T) {
	cheCluster := &orgv1.CheCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "eclipse-che",
			Namespace: "eclipse-che-1",
		},
		Spec: orgv1.CheClusterSpec{
			DevWorkspace: orgv1.CheClusterSpecDevWorkspace{
				Enable: true,
			},
		},
	}
	anotherCheCluster := &orgv1.CheCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "eclipse-che",
			Namespace: "eclipse-che-2",
		},
		Spec: orgv1.CheClusterSpec{
			DevWorkspace: orgv1.CheClusterSpecDevWorkspace{
				Enable: true,
			},
		},
	}
	deployContext := deploy.GetTestDeployContext(cheCluster, []runtime.Object{anotherCheCluster})

	// creates initial object
	initialObject := deploy.GetConfigMapSpec(deployContext, "test", map[string]string{"a": "b"}, "test")
	initialObject.SetAnnotations(map[string]string{
		deploy.CheEclipseOrgHash256:   "hash-2",
		deploy.CheEclipseOrgNamespace: "eclipse-che-2",
	})
	isCreated, err := deploy.Create(deployContext, initialObject)
	assert.NoError(t, err)
	assert.True(t, isCreated)

	// tries to sync object with a new hash but different origin
	newObject := deploy.GetConfigMapSpec(deployContext, "test", map[string]string{"a": "c"}, "test")
	newObject.GetAnnotations()[deploy.CheEclipseOrgHash256] = "newHash"
	isDone, err := syncObject(deployContext, newObject, "eclipse-che")
	assert.NoError(t, err, "Failed to sync object")
	assert.True(t, isDone, "Failed to sync object")

	// reads object and check content, object isn't supposed to be updated
	actual := &corev1.ConfigMap{}
	exists, err := deploy.GetNamespacedObject(deployContext, "test", actual)
	assert.NoError(t, err, "failed to get configmap")
	assert.True(t, exists, "configmap is not found")

	assert.Equal(t, "eclipse-che-2", actual.GetAnnotations()[deploy.CheEclipseOrgNamespace], "hash annotation mismatch")
	assert.Equal(t, "hash-2", actual.GetAnnotations()[deploy.CheEclipseOrgHash256], "namespace annotation mismatch")
	assert.Equal(t, "b", actual.Data["a"], "data mismatch")
}

func TestShouldNotSyncObjectIfHashIsEqual(t *testing.T) {
	deployContext := deploy.GetTestDeployContext(nil, []runtime.Object{})

	// creates initial object
	initialObject := deploy.GetConfigMapSpec(deployContext, "test", map[string]string{"a": "b"}, "test")
	initialObject.SetAnnotations(map[string]string{
		deploy.CheEclipseOrgHash256:   "hash",
		deploy.CheEclipseOrgNamespace: "eclipse-che",
	})
	isCreated, err := deploy.Create(deployContext, initialObject)
	assert.NoError(t, err)
	assert.True(t, isCreated)

	// tries to sync object with the same hash
	newObject := deploy.GetConfigMapSpec(deployContext, "test", map[string]string{"a": "c"}, "test")
	newObject.GetAnnotations()[deploy.CheEclipseOrgHash256] = "hash"

	isDone, err := syncObject(deployContext, newObject, "eclipse-che")
	assert.NoError(t, err, "Failed to sync object")
	assert.True(t, isDone, "Failed to sync object")

	// reads object and check content, object isn't supposed to be updated
	actual := &corev1.ConfigMap{}
	exists, err := deploy.GetNamespacedObject(deployContext, "test", actual)
	assert.NoError(t, err, "failed to get configmap")
	assert.True(t, exists, "configmap is not found")

	assert.Equal(t, "eclipse-che", actual.GetAnnotations()[deploy.CheEclipseOrgNamespace], "hash annotation mismatch")
	assert.Equal(t, "hash", actual.GetAnnotations()[deploy.CheEclipseOrgHash256], "namespace annotation mismatch")
	assert.Equal(t, "b", actual.Data["a"], "data mismatch")
}

func TestShouldNotSyncObjectIfNamespaceIsNotManagedByChe(t *testing.T) {
	deployContext := deploy.GetTestDeployContext(nil, []runtime.Object{})

	namespace := &corev1.Namespace{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Namespace",
			APIVersion: corev1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:        DevWorkspaceNamespace,
			Annotations: map[string]string{
				// no che annotation is here
			},
		},
		Spec: corev1.NamespaceSpec{},
	}
	isCreated, err := deploy.Create(deployContext, namespace)
	assert.NoError(t, err)
	assert.True(t, isCreated)

	initialObject := deploy.GetConfigMapSpec(deployContext, "test", map[string]string{"a": "b"}, "test")
	initialObject.SetAnnotations(map[string]string{})
	isCreated, err = deploy.Create(deployContext, initialObject)
	assert.NoError(t, err)
	assert.True(t, isCreated)

	// creates initial object
	newObject := deploy.GetConfigMapSpec(deployContext, "test", map[string]string{"a": "c"}, "test")
	newObject.GetAnnotations()[deploy.CheEclipseOrgHash256] = "newHash"

	// tries to sync object with a new
	_, err = syncObject(deployContext, newObject, DevWorkspaceNamespace)
	assert.NoError(t, err)

	// reads object and check content, object supposed to be updated
	// it was created by the same origin
	actual := &corev1.ConfigMap{}
	exists, err := deploy.GetNamespacedObject(deployContext, "test", actual)
	assert.NoError(t, err, "failed to get configmap")
	assert.True(t, exists, "configmap is not found")

	assert.Equal(t, "", actual.GetAnnotations()[deploy.CheEclipseOrgNamespace], "hash annotation mismatch")
	assert.Equal(t, "", actual.GetAnnotations()[deploy.CheEclipseOrgHash256], "namespace annotation mismatch")
	assert.Equal(t, "b", actual.Data["a"], "data mismatch")
}

func TestShouldSyncObjectIfItWasCreatedByAnotherOriginHashDifferent(t *testing.T) {
	cheCluster := &orgv1.CheCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "eclipse-che",
			Namespace: "eclipse-che",
		},
		Spec: orgv1.CheClusterSpec{
			DevWorkspace: orgv1.CheClusterSpecDevWorkspace{
				Enable: true,
			},
		},
	}
	deployContext := deploy.GetTestDeployContext(cheCluster, []runtime.Object{})

	// creates initial object
	initialObject := deploy.GetConfigMapSpec(deployContext, "test", map[string]string{"a": "b"}, "test")
	initialObject.SetAnnotations(map[string]string{
		deploy.CheEclipseOrgHash256:   "hash2",
		deploy.CheEclipseOrgNamespace: "eclipse-che-2",
	})
	isCreated, err := deploy.Create(deployContext, initialObject)
	assert.NoError(t, err)
	assert.True(t, isCreated)

	// tries to sync object with a new hash but different origin
	newObject := deploy.GetConfigMapSpec(deployContext, "test", map[string]string{"a": "c"}, "test")
	newObject.GetAnnotations()[deploy.CheEclipseOrgHash256] = "hash"

	_, err = syncObject(deployContext, newObject, "eclipse-che")
	assert.NoError(t, err, "Failed to sync object")

	// reads object and check content, object supposed to be updated
	// there is only one operator to mange DW resources
	actual := &corev1.ConfigMap{}
	exists, err := deploy.GetNamespacedObject(deployContext, "test", actual)
	assert.NoError(t, err, "failed to get configmap")
	assert.True(t, exists, "configmap is not found")

	assert.Equal(t, "eclipse-che", actual.GetAnnotations()[deploy.CheEclipseOrgNamespace], "hash annotation mismatch")
	assert.Equal(t, "hash", actual.GetAnnotations()[deploy.CheEclipseOrgHash256], "namespace annotation mismatch")
	assert.Equal(t, "c", actual.Data["a"], "data mismatch")
}
