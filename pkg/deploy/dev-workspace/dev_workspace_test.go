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
package devworkspace

import (
	"context"

	orgv1 "github.com/eclipse-che/che-operator/pkg/apis/org/v1"
	"github.com/eclipse-che/che-operator/pkg/deploy"
	"github.com/eclipse-che/che-operator/pkg/util"
	operatorsv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	fakeDiscovery "k8s.io/client-go/discovery/fake"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"testing"
)

func TestReconcileDevWorkspace(t *testing.T) {
	cheCluster := &orgv1.CheCluster{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "eclipse-che",
		},
		Spec: orgv1.CheClusterSpec{
			DevWorkspace: orgv1.CheClusterSpecDevWorkspace{
				Enable: true,
			},
			Auth: orgv1.CheClusterSpecAuth{
				OpenShiftoAuth: util.NewBoolPointer(true),
			},
			Server: orgv1.CheClusterSpecServer{
				ServerExposureStrategy: "single-host",
			},
		},
	}

	deployContext := deploy.GetTestDeployContext(cheCluster, []runtime.Object{})
	deployContext.ClusterAPI.Scheme.AddKnownTypes(operatorsv1alpha1.SchemeGroupVersion, &operatorsv1alpha1.Subscription{})
	deployContext.ClusterAPI.DiscoveryClient.(*fakeDiscovery.FakeDiscovery).Fake.Resources = []*metav1.APIResourceList{
		{
			APIResources: []metav1.APIResource{
				{Name: CheManagerResourcename},
			},
		},
	}

	util.IsOpenShift4 = true
	done, err := ReconcileDevWorkspace(deployContext)

	if err != nil {
		t.Fatalf("Error: %v", err)
	}

	if !done {
		t.Fatalf("Dev Workspace operator has not been provisioned")
	}

	t.Run("defaultCheManagerDeployed", func(t *testing.T) {
		obj := &unstructured.Unstructured{}
		obj.SetGroupVersionKind(schema.GroupVersionKind{Group: "che.eclipse.org", Version: "v1alpha1", Kind: "CheManager"})
		err := deployContext.ClusterAPI.Client.Get(context.TODO(), client.ObjectKey{Name: "devworkspace-che", Namespace: DevWorkspaceCheNamespace}, obj)
		if err != nil {
			t.Fatalf("Should have found a CheManager with default config but got an error: %s", err)
		}

		if obj.GetName() != "devworkspace-che" {
			t.Fatalf("Should have found a CheManager with default config but found: %s", obj.GetName())
		}
	})
}

func TestReconcileDevWorkspaceShouldThrowErrorIfWebTerminalSubscriptionExists(t *testing.T) {
	cheCluster := &orgv1.CheCluster{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "eclipse-che",
		},
		Spec: orgv1.CheClusterSpec{
			DevWorkspace: orgv1.CheClusterSpecDevWorkspace{
				Enable: true,
			},
			Auth: orgv1.CheClusterSpecAuth{
				OpenShiftoAuth: util.NewBoolPointer(true),
			},
			Server: orgv1.CheClusterSpecServer{
				ServerExposureStrategy: "single-host",
			},
		},
	}
	subscription := &operatorsv1alpha1.Subscription{
		ObjectMeta: metav1.ObjectMeta{
			Name:      WebTerminalOperatorSubscriptionName,
			Namespace: WebTerminalOperatorNamespace,
		},
		Spec: &operatorsv1alpha1.SubscriptionSpec{},
	}
	webhook := &admissionregistrationv1.MutatingWebhookConfiguration{
		ObjectMeta: metav1.ObjectMeta{
			Name: DevWorkspaceWebhookName,
		},
	}

	deployContext := deploy.GetTestDeployContext(cheCluster, []runtime.Object{subscription, webhook})
	deployContext.ClusterAPI.Scheme.AddKnownTypes(operatorsv1alpha1.SchemeGroupVersion, &operatorsv1alpha1.Subscription{})
	deployContext.ClusterAPI.Scheme.AddKnownTypes(admissionregistrationv1.SchemeGroupVersion, &admissionregistrationv1.MutatingWebhookConfiguration{})
	deployContext.ClusterAPI.DiscoveryClient.(*fakeDiscovery.FakeDiscovery).Fake.Resources = []*metav1.APIResourceList{
		{
			APIResources: []metav1.APIResource{
				{Name: SubscriptionResourceName},
			},
		},
	}

	util.IsOpenShift4 = true
	_, err := ReconcileDevWorkspace(deployContext)

	if err == nil || err.Error() != "A non matching version of the Dev Workspace operator is already installed" {
		t.Fatalf("Error should be thrown")
	}
}

func TestShouldSyncObject(t *testing.T) {
	deployContext := deploy.GetTestDeployContext(nil, []runtime.Object{})

	testObject := deploy.GetConfigMapSpec(deployContext, "test", map[string]string{}, "test")
	obj2sync := &Object2Sync{
		obj:     testObject,
		hash256: "hash1",
	}

	done, err := syncObject(deployContext, obj2sync)
	if err != nil {
		t.Fatalf("Failed to sync object: %v", err)
	} else if !done {
		t.Fatalf("Object is not synced.")
	}

	actual := &corev1.ConfigMap{}
	exists, err := deploy.GetNamespacedObject(deployContext, "test", actual)
	if err != nil {
		t.Fatalf("Failed to get object: %v", err)
	} else if !exists {
		t.Fatalf("Object not found")
	}

	if actual.GetAnnotations()[deploy.CheEclipseOrgHash256] != obj2sync.hash256 {
		t.Fatalf("Invalid hash")
	}
	if actual.GetAnnotations()[deploy.CheEclipseOrgCreatedBy] != getCreatedBy(deployContext) {
		t.Fatalf("Invalid created-by")
	}
}

func TestShouldSyncObjectIfHashIsNotEqual(t *testing.T) {
	deployContext := deploy.GetTestDeployContext(nil, []runtime.Object{})

	initialObject := deploy.GetConfigMapSpec(deployContext, "test", map[string]string{"a": "b"}, "test")
	initialObject.SetAnnotations(map[string]string{
		deploy.CheEclipseOrgHash256:   "hash1",
		deploy.CheEclipseOrgCreatedBy: getCreatedBy(deployContext),
	})
	deploy.Create(deployContext, initialObject)

	testObject := deploy.GetConfigMapSpec(deployContext, "test", map[string]string{"a": "c"}, "test")
	obj2sync := &Object2Sync{
		obj:     testObject,
		hash256: "hash2",
	}

	done, err := syncObject(deployContext, obj2sync)
	if err != nil {
		t.Fatalf("Failed to sync object: %v", err)
	} else if done {
		t.Fatalf("Object is updated, another sync is required.")
	}

	done, err = syncObject(deployContext, obj2sync)
	if err != nil {
		t.Fatalf("Failed to sync object: %v", err)
	} else if !done {
		t.Fatalf("Object is not synced.")
	}

	actual := &corev1.ConfigMap{}
	exists, err := deploy.GetNamespacedObject(deployContext, "test", actual)
	if err != nil {
		t.Fatalf("Failed to get object: %v", err)
	} else if !exists {
		t.Fatalf("Object not found")
	}

	if actual.GetAnnotations()[deploy.CheEclipseOrgHash256] != obj2sync.hash256 {
		t.Fatalf("Invalid hash")
	}
	if actual.GetAnnotations()[deploy.CheEclipseOrgCreatedBy] != getCreatedBy(deployContext) {
		t.Fatalf("Invalid created-by")
	}
	if actual.Data["a"] == "b" {
		t.Fatalf("Object is not updated.")
	}
}

func TestShouldNotSyncObjectIfHashIsEqual(t *testing.T) {
	deployContext := deploy.GetTestDeployContext(nil, []runtime.Object{})

	initialObject := deploy.GetConfigMapSpec(deployContext, "test", map[string]string{"a": "b"}, "test")
	initialObject.SetAnnotations(map[string]string{
		deploy.CheEclipseOrgHash256:   "hash1",
		deploy.CheEclipseOrgCreatedBy: getCreatedBy(deployContext),
	})
	deploy.Create(deployContext, initialObject)

	testObject := deploy.GetConfigMapSpec(deployContext, "test", map[string]string{"a": "c"}, "test")
	obj2sync := &Object2Sync{
		obj:     testObject,
		hash256: "hash1",
	}

	done, err := syncObject(deployContext, obj2sync)
	if err != nil {
		t.Fatalf("Failed to sync object: %v", err)
	} else if !done {
		t.Fatalf("Object is not synced.")
	}

	actual := &corev1.ConfigMap{}
	exists, err := deploy.GetNamespacedObject(deployContext, "test", actual)
	if err != nil {
		t.Fatalf("Failed to get object: %v", err)
	} else if !exists {
		t.Fatalf("Object not found")
	}

	if actual.Data["a"] != "b" {
		t.Fatalf("Object is not supposed to be updated.")
	}
}

func TestShouldNotSyncObjectIfCreatedByIsDifferent(t *testing.T) {
	deployContext := deploy.GetTestDeployContext(nil, []runtime.Object{})

	initialObject := deploy.GetConfigMapSpec(deployContext, "test", map[string]string{"a": "b"}, "test")
	initialObject.SetAnnotations(map[string]string{
		deploy.CheEclipseOrgHash256: "hash1",
	})
	deploy.Create(deployContext, initialObject)

	testObject := deploy.GetConfigMapSpec(deployContext, "test", map[string]string{"a": "c"}, "test")
	obj2sync := &Object2Sync{
		obj:     testObject,
		hash256: "hash1",
	}

	done, err := syncObject(deployContext, obj2sync)
	if err != nil {
		t.Fatalf("Failed to sync object: %v", err)
	} else if !done {
		t.Fatalf("Object is not synced.")
	}

	actual := &corev1.ConfigMap{}
	exists, err := deploy.GetNamespacedObject(deployContext, "test", actual)
	if err != nil {
		t.Fatalf("Failed to get object: %v", err)
	} else if !exists {
		t.Fatalf("Object not found")
	}

	if actual.Data["a"] != "b" {
		t.Fatalf("Object is not supposed to be updated.")
	}
}
