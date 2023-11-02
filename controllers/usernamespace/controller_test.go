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

package usernamespace

import (
	"context"
	"encoding/json"
	"sync"
	"testing"

	containerbuild "github.com/eclipse-che/che-operator/pkg/deploy/container-build"
	rbacv1 "k8s.io/api/rbac/v1"

	dwconstants "github.com/devfile/devworkspace-operator/pkg/constants"
	"github.com/devfile/devworkspace-operator/pkg/infrastructure"
	devworkspaceinfra "github.com/devfile/devworkspace-operator/pkg/infrastructure"
	chev2 "github.com/eclipse-che/che-operator/api/v2"
	"github.com/eclipse-che/che-operator/controllers/devworkspace"
	"github.com/eclipse-che/che-operator/pkg/common/constants"
	"github.com/eclipse-che/che-operator/pkg/deploy/tls"
	configv1 "github.com/openshift/api/config/v1"
	projectv1 "github.com/openshift/api/project/v1"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func setupCheCluster(t *testing.T, ctx context.Context, cl client.Client, scheme *runtime.Scheme, cheNamespaceName string, cheName string) {
	var cheNamespace metav1.Object
	if infrastructure.IsOpenShift() {
		cheNamespace = &projectv1.Project{}
	} else {
		cheNamespace = &corev1.Namespace{}
	}

	cheNamespace.SetName(cheNamespaceName)
	if err := cl.Create(ctx, cheNamespace.(client.Object)); err != nil {
		t.Fatal(err)
	}

	cheCluster := chev2.CheCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cheName,
			Namespace: cheNamespaceName,
		},
		Spec: chev2.CheClusterSpec{
			DevEnvironments: chev2.CheClusterDevEnvironments{
				DisableContainerBuildCapabilities: pointer.BoolPtr(true),
				NodeSelector:                      map[string]string{"a": "b", "c": "d"},
				Tolerations: []corev1.Toleration{
					{
						Key:      "a",
						Operator: corev1.TolerationOpEqual,
						Value:    "b",
					},
					{
						Key:      "c",
						Operator: corev1.TolerationOpEqual,
						Value:    "d",
					},
				},
				TrustedCerts: &chev2.TrustedCerts{
					GitTrustedCertsConfigMapName: "che-git-self-signed-cert",
				},
				SecondsOfInactivityBeforeIdling: pointer.Int32Ptr(1800),
				SecondsOfRunBeforeIdling:        pointer.Int32Ptr(-1),
			},
			Networking: chev2.CheClusterSpecNetworking{
				Domain: "root-domain",
			},
		},
		Status: chev2.CheClusterStatus{
			CheURL: "https://che-host",
		},
	}
	if err := cl.Create(ctx, &cheCluster); err != nil {
		t.Fatal(err)
	}

	// also create the self-signed-certificate secret to pretend we have TLS set up
	cert := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      constants.DefaultSelfSignedCertificateSecretName,
			Namespace: cheNamespaceName,
		},
		Data: map[string][]byte{
			"ca.crt":     []byte("my certificate"),
			"other.data": []byte("should not be copied to target ns"),
		},
		Type:      "Opaque",
		Immutable: pointer.BoolPtr(true),
	}
	if err := cl.Create(ctx, cert); err != nil {
		t.Fatal(err)
	}

	caCerts := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      tls.CheAllCACertsConfigMapName,
			Namespace: cheNamespaceName,
		},
		Data: map[string]string{
			"trusted1": "trusted cert 1",
			"trusted2": "trusted cert 2",
		},
	}
	if err := cl.Create(ctx, caCerts); err != nil {
		t.Fatal(err)
	}

	gitTlsCredentials := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "che-git-self-signed-cert",
			Namespace: cheNamespaceName,
		},
		Data: map[string]string{
			"githost": "the.host.of.git",
			"ca.crt":  "the public certificate of the.host.of.git",
		},
	}
	if err := cl.Create(ctx, gitTlsCredentials); err != nil {
		t.Fatal(err)
	}

	r := devworkspace.New(cl, scheme)
	// the reconciliation needs to run twice for it to be truly finished - we're setting up finalizers etc...
	if _, err := r.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: cheName, Namespace: cheNamespaceName}}); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: cheName, Namespace: cheNamespaceName}}); err != nil {
		t.Fatal(err)
	}
}

func setup(infraType devworkspaceinfra.Type, objs ...runtime.Object) (*runtime.Scheme, client.Client, *CheUserNamespaceReconciler) {
	devworkspaceinfra.InitializeForTesting(infraType)
	devworkspace.CleanCheClusterInstancesForTest()
	infrastructure.InitializeForTesting(infraType)

	scheme := createTestScheme()

	cl := fake.NewFakeClientWithScheme(scheme, objs...)

	r := &CheUserNamespaceReconciler{
		client: cl,
		scheme: scheme,
		namespaceCache: namespaceCache{
			client:          cl,
			knownNamespaces: map[string]namespaceInfo{},
			lock:            sync.Mutex{},
		},
	}

	return scheme, cl, r
}

func TestSkipsUnlabeledNamespaces(t *testing.T) {
	test := func(t *testing.T, infraType devworkspaceinfra.Type, namespace metav1.Object) {
		ctx := context.TODO()
		scheme, cl, r := setup(infraType, namespace.(runtime.Object))
		setupCheCluster(t, ctx, cl, scheme, "che", "che")

		if _, err := r.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: namespace.GetName()}}); err != nil {
			t.Fatal(err)
		}

		// no new secret or configmap should be created in the namespace
		ss := &corev1.SecretList{}
		if err := cl.List(ctx, ss, client.InNamespace(namespace.GetName())); err != nil {
			t.Fatal(err)
		}

		assert.True(t, len(ss.Items) == 0, "No secrets expected in the tested namespace but found %d", len(ss.Items))

		cs := &corev1.ConfigMapList{}
		if err := cl.List(ctx, cs, client.InNamespace(namespace.GetName())); err != nil {
			t.Fatal(err)
		}
		assert.True(t, len(cs.Items) == 0, "No configmaps expected in the tested namespace but found %d", len(cs.Items))
	}

	t.Run("k8s", func(t *testing.T) {
		test(t, devworkspaceinfra.Kubernetes, &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: "ns",
			},
		})
	})

	t.Run("openshift", func(t *testing.T) {
		test(t, devworkspaceinfra.OpenShiftv4, &projectv1.Project{
			ObjectMeta: metav1.ObjectMeta{
				Name: "prj",
			},
		})
	})
}

func TestRequiresLabelsToMatchOneOfMultipleCheCluster(t *testing.T) {
	test := func(t *testing.T, infraType devworkspaceinfra.Type, namespace metav1.Object) {
		ctx := context.TODO()
		scheme, cl, r := setup(infraType, namespace.(runtime.Object))
		setupCheCluster(t, ctx, cl, scheme, "che1", "che")
		setupCheCluster(t, ctx, cl, scheme, "che2", "che")

		res, err := r.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: namespace.GetName()}})
		assert.NoError(t, err, "Reconciliation should have succeeded.")

		assert.True(t, res.Requeue, "The reconciliation request should have been requeued.")
	}

	t.Run("k8s", func(t *testing.T) {
		test(t, devworkspaceinfra.Kubernetes, &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: "ns",
				Labels: map[string]string{
					workspaceNamespaceOwnerUidLabel: "uid",
				},
			},
		})
	})

	t.Run("openshift", func(t *testing.T) {
		test(t, devworkspaceinfra.OpenShiftv4, &projectv1.Project{
			ObjectMeta: metav1.ObjectMeta{
				Name: "prj",
				Labels: map[string]string{
					workspaceNamespaceOwnerUidLabel: "uid",
				},
			},
		})
	})
}

func TestMatchingCheClusterCanBeSelectedUsingLabels(t *testing.T) {
	test := func(t *testing.T, infraType devworkspaceinfra.Type, namespace string, objs ...runtime.Object) {
		ctx := context.TODO()
		scheme, cl, r := setup(infraType, objs...)
		setupCheCluster(t, ctx, cl, scheme, "che1", "che")
		setupCheCluster(t, ctx, cl, scheme, "che2", "che")

		res, err := r.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: namespace}})
		assert.NoError(t, err, "Reconciliation shouldn't have failed")

		assert.False(t, res.Requeue, "The reconciliation request should have succeeded but is requesting a requeue.")
	}

	t.Run("k8s", func(t *testing.T) {
		test(t, devworkspaceinfra.Kubernetes,
			"ns",
			&corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "ns",
					Labels: map[string]string{
						workspaceNamespaceOwnerUidLabel: "uid",
						cheNameLabel:                    "che",
						cheNamespaceLabel:               "che1",
					},
				},
			})
	})

	t.Run("openshift", func(t *testing.T) {
		test(t, devworkspaceinfra.OpenShiftv4,
			"ns",
			&corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "ns",
					Labels: map[string]string{
						workspaceNamespaceOwnerUidLabel: "uid",
						cheNameLabel:                    "che",
						cheNamespaceLabel:               "che1",
					},
				},
			},
			&projectv1.Project{
				ObjectMeta: metav1.ObjectMeta{
					Name: "prj",
					Labels: map[string]string{
						workspaceNamespaceOwnerUidLabel: "uid",
						cheNameLabel:                    "che",
						cheNamespaceLabel:               "che1",
					},
				},
			})
	})
}

func TestCreatesDataInNamespace(t *testing.T) {
	infrastructure.InitializeForTesting(infrastructure.Kubernetes)

	expectedPodTolerations, err := json.Marshal([]corev1.Toleration{
		{
			Key:      "a",
			Operator: corev1.TolerationOpEqual,
			Value:    "b",
		},
		{
			Key:      "c",
			Operator: corev1.TolerationOpEqual,
			Value:    "d",
		},
	})
	assert.NoError(t, err)

	test := func(t *testing.T, infraType devworkspaceinfra.Type, namespace client.Object, objs ...runtime.Object) {
		ctx := context.TODO()
		allObjs := append(objs, namespace.(runtime.Object))
		scheme, cl, r := setup(infraType, allObjs...)
		setupCheCluster(t, ctx, cl, scheme, "eclipse-che", "che")

		res, err := r.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: namespace.GetName()}})
		assert.NoError(t, err, "Reconciliation should have succeeded")

		assert.False(t, res.Requeue, "The reconciliation request should have succeeded but it is requesting a requeue")

		proxySettings := corev1.ConfigMap{}
		assert.NoError(t, cl.Get(ctx, client.ObjectKey{Name: "che-proxy-settings", Namespace: namespace.GetName()}, &proxySettings))

		assert.Equal(t, "env", proxySettings.GetAnnotations()[dwconstants.DevWorkspaceMountAsAnnotation],
			"proxy settings should be annotated as mount as 'env'")

		assert.Equal(t, "true", proxySettings.GetLabels()[dwconstants.DevWorkspaceMountLabel],
			"proxy settings should be labeled as mounted")

		assert.Equal(t, 2, len(proxySettings.Data), "Expecting 2 elements in the default proxy settings")

		assert.Equal(t, ".svc", proxySettings.Data["NO_PROXY"], "Unexpected proxy settings")

		idleSettings := corev1.ConfigMap{}
		assert.NoError(t, cl.Get(ctx, client.ObjectKey{Name: "che-idle-settings", Namespace: namespace.GetName()}, &idleSettings))

		assert.Equal(t, "env", idleSettings.GetAnnotations()[dwconstants.DevWorkspaceMountAsAnnotation],
			"idle settings should be annotated as mount as 'env'")

		assert.Equal(t, "true", idleSettings.GetLabels()[dwconstants.DevWorkspaceMountLabel],
			"idle settings should be labeled as mounted")

		assert.Equal(t, 2, len(idleSettings.Data), "Expecting 2 elements in the idle settings")

		assert.Equal(t, "1800", idleSettings.Data["SECONDS_OF_DW_INACTIVITY_BEFORE_IDLING"], "Unexpected idle settings")
		assert.Equal(t, "-1", idleSettings.Data["SECONDS_OF_DW_RUN_BEFORE_IDLING"], "Unexpected idle settings")

		cert := corev1.Secret{}
		assert.NoError(t, cl.Get(ctx, client.ObjectKey{Name: "che-server-cert", Namespace: namespace.GetName()}, &cert))

		assert.Equal(t, "file", cert.GetAnnotations()[dwconstants.DevWorkspaceMountAsAnnotation], "server cert should be annotated as mount as 'file'")
		assert.Equal(t, "/tmp/che/secret/", cert.GetAnnotations()[dwconstants.DevWorkspaceMountPathAnnotation], "server cert annotated as mounted to an unexpected path")
		assert.Equal(t, "true", cert.GetLabels()[dwconstants.DevWorkspaceMountLabel], "server cert should be labeled as mounted")
		assert.Equal(t, 1, len(cert.Data), "Expecting just 1 element in the self-signed cert")
		assert.Equal(t, "my certificate", string(cert.Data["ca.crt"]), "Unexpected self-signed certificate")
		assert.Equal(t, corev1.SecretTypeOpaque, cert.Type, "Unexpected secret type")
		assert.Equal(t, true, *cert.Immutable, "Unexpected mutability of the secret")

		caCerts := corev1.ConfigMap{}
		assert.NoError(t, cl.Get(ctx, client.ObjectKey{Name: "che-trusted-ca-certs", Namespace: namespace.GetName()}, &caCerts))
		assert.Equal(t, "file", caCerts.GetAnnotations()[dwconstants.DevWorkspaceMountAsAnnotation], "trusted certs should be annotated as mount as 'file'")
		assert.Equal(t, "/public-certs", caCerts.GetAnnotations()[dwconstants.DevWorkspaceMountPathAnnotation], "trusted certs annotated as mounted to an unexpected path")
		assert.Equal(t, "true", caCerts.GetLabels()[dwconstants.DevWorkspaceMountLabel], "trusted certs should be labeled as mounted")
		assert.Equal(t, 2, len(caCerts.Data), "Expecting exactly 2 data entries in the trusted cert config map")
		assert.Equal(t, "trusted cert 1", string(caCerts.Data["trusted1"]), "Unexpected trusted cert 1 value")
		assert.Equal(t, "trusted cert 2", string(caCerts.Data["trusted2"]), "Unexpected trusted cert 2 value")

		gitTlsConfig := corev1.ConfigMap{}
		assert.NoError(t, cl.Get(ctx, client.ObjectKey{Name: "che-git-tls-creds", Namespace: namespace.GetName()}, &gitTlsConfig))
		assert.Equal(t, "true", gitTlsConfig.Labels[dwconstants.DevWorkspaceGitTLSLabel])
		assert.Equal(t, "true", gitTlsConfig.Labels[dwconstants.DevWorkspaceMountLabel])
		assert.Equal(t, "true", gitTlsConfig.Labels[dwconstants.DevWorkspaceWatchConfigMapLabel])
		assert.Equal(t, "the.host.of.git", gitTlsConfig.Data["host"])
		assert.Equal(t, "the public certificate of the.host.of.git", gitTlsConfig.Data["certificate"])

		updatedNs := namespace.DeepCopyObject().(client.Object)
		assert.NoError(t, cl.Get(ctx, client.ObjectKeyFromObject(namespace), updatedNs))
		assert.Equal(t, `{"a":"b","c":"d"}`, updatedNs.GetAnnotations()[nodeSelectorAnnotation])
		assert.Equal(t, string(expectedPodTolerations), updatedNs.GetAnnotations()[podTolerationsAnnotation])
	}

	t.Run("k8s", func(t *testing.T) {
		test(t, devworkspaceinfra.Kubernetes, &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: "ns",
				Labels: map[string]string{
					workspaceNamespaceOwnerUidLabel: "uid",
				},
			},
		})
	})

	t.Run("openshift", func(t *testing.T) {
		test(t, devworkspaceinfra.OpenShiftv4,
			&corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "prj",
					Labels: map[string]string{
						workspaceNamespaceOwnerUidLabel: "uid",
					},
				},
			},
			&projectv1.Project{
				ObjectMeta: metav1.ObjectMeta{
					Name: "prj",
					Labels: map[string]string{
						workspaceNamespaceOwnerUidLabel: "uid",
					},
				},
			}, &configv1.Proxy{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cluster",
				},
				Spec: configv1.ProxySpec{
					NoProxy: ".svc",
				},
				Status: configv1.ProxyStatus{
					NoProxy: ".svc",
				},
			})
	})
}

func TestUpdateSccClusterRoleBinding(t *testing.T) {
	infrastructure.InitializeForTesting(infrastructure.OpenShiftv4)

	pr1 := &projectv1.Project{
		ObjectMeta: metav1.ObjectMeta{
			Name: "ns1",
			Labels: map[string]string{
				workspaceNamespaceOwnerUidLabel: "uid_1",
			},
			Annotations: map[string]string{
				cheUsernameAnnotation: "user_1",
			},
		},
	}

	ns1 := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "ns1",
			Labels: map[string]string{
				workspaceNamespaceOwnerUidLabel: "uid_1",
			},
			Annotations: map[string]string{
				cheUsernameAnnotation: "user_1",
			},
		},
	}

	cheCluster := &chev2.CheCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "eclipse-che",
			Namespace: "eclipse-che",
		},
		Spec: chev2.CheClusterSpec{
			DevEnvironments: chev2.CheClusterDevEnvironments{
				DisableContainerBuildCapabilities: pointer.BoolPtr(false),
				ContainerBuildConfiguration: &chev2.ContainerBuildConfiguration{
					OpenShiftSecurityContextConstraint: "container-build",
				},
			},
			Networking: chev2.CheClusterSpecNetworking{
				Domain: "root-domain",
			},
		},
		Status: chev2.CheClusterStatus{
			CheURL: "https://che-host",
		},
	}

	allObjs := []runtime.Object{ns1, pr1, cheCluster}
	scheme, cl, usernamespaceReconciler := setup(infrastructure.OpenShiftv4, allObjs...)

	// the reconciliation needs to run twice for it to be truly finished - we're setting up finalizers etc...
	devworkspaceReconciler := devworkspace.New(cl, scheme)
	if _, err := devworkspaceReconciler.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: "eclipse-che", Namespace: "eclipse-che"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := devworkspaceReconciler.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: "eclipse-che", Namespace: "eclipse-che"}}); err != nil {
		t.Fatal(err)
	}

	_, err := usernamespaceReconciler.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: ns1.GetName()}})
	assert.Nil(t, err)

	rb := &rbacv1.RoleBinding{}
	err = cl.Get(context.TODO(), types.NamespacedName{Name: containerbuild.GetUserSccRbacResourcesName(), Namespace: "ns1"}, rb)
	assert.Nil(t, err)
	assert.Equal(t, "user_1", rb.Subjects[0].Name)
}

func TestWatchRulesForSecretsInSameNamespace(t *testing.T) {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "sec",
			Namespace: "ns",
			Labels:    map[string]string{"app.kubernetes.io/component": "user-settings"},
		},
	}

	_, _, r := setup(devworkspaceinfra.Kubernetes, &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "ns",
			Labels: map[string]string{
				workspaceNamespaceOwnerUidLabel: "uid",
			},
		},
	}, secret)

	ctx := context.TODO()

	h := r.watchRulesForSecrets(ctx)
	rlq := workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter())
	// Let's throw event to controller about new secret creation.
	h.Create(event.CreateEvent{Object: secret}, rlq)

	amountReconcileRequests := rlq.Len()
	rs, _ := rlq.Get()

	assert.Equal(t, 1, amountReconcileRequests)
	assert.Equal(t, "ns", rs.(reconcile.Request).Name)
}

func TestWatchRulesForConfigMapsInSameNamespace(t *testing.T) {
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "cm",
			Namespace: "ns",
			Labels:    map[string]string{"app.kubernetes.io/component": "user-settings"},
		},
	}

	_, _, r := setup(devworkspaceinfra.Kubernetes, &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "ns",
			Labels: map[string]string{
				workspaceNamespaceOwnerUidLabel: "uid",
			},
		},
	}, cm)

	ctx := context.TODO()

	h := r.watchRulesForSecrets(ctx)
	rlq := workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter())
	// Let's throw event to controller about new config map creation.
	h.Create(event.CreateEvent{Object: cm}, rlq)

	amountReconcileRequests := rlq.Len()
	rs, _ := rlq.Get()

	assert.Equal(t, 1, amountReconcileRequests)
	assert.Equal(t, "ns", rs.(reconcile.Request).Name)
}

func TestWatchRulesForSecretsInOtherNamespaces(t *testing.T) {
	secret := &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Secret",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      constants.DefaultSelfSignedCertificateSecretName,
			Namespace: "eclipse-che",
		},
	}

	_, _, r := setup(devworkspaceinfra.Kubernetes,
		&corev1.Namespace{
			TypeMeta: metav1.TypeMeta{
				Kind:       "Namespace",
				APIVersion: "v1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: "ns1",
				Labels: map[string]string{
					workspaceNamespaceOwnerUidLabel: "uid1",
				},
			},
		},
		&corev1.Namespace{
			TypeMeta: metav1.TypeMeta{
				Kind:       "Namespace",
				APIVersion: "v1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: "ns2",
				Labels: map[string]string{
					workspaceNamespaceOwnerUidLabel: "uid2",
				},
			},
		},
		&corev1.Namespace{
			TypeMeta: metav1.TypeMeta{
				Kind:       "Namespace",
				APIVersion: "v1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: "eclipse-che",
			},
		},
		&chev2.CheCluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "che",
				Namespace: "eclipse-che",
			},
		},
		secret)

	ctx := context.TODO()

	r.namespaceCache.ExamineNamespace(ctx, "ns1")
	r.namespaceCache.ExamineNamespace(ctx, "ns2")
	r.namespaceCache.ExamineNamespace(ctx, "eclipse-che")

	h := r.watchRulesForSecrets(ctx)
	rlq := workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter())
	// Let's throw event to controller about new secret creation.
	h.Create(event.CreateEvent{Object: secret}, rlq)

	amountReconcileRequests := rlq.Len()
	rs1, _ := rlq.Get()
	rs2, _ := rlq.Get()
	rs3, _ := rlq.Get()
	reconciles := []reconcile.Request{rs1.(reconcile.Request), rs2.(reconcile.Request), rs3.(reconcile.Request)}

	assert.Equal(t, 3, amountReconcileRequests)
	assert.Contains(t, reconciles, reconcile.Request{NamespacedName: types.NamespacedName{Name: "ns1"}})
	assert.Contains(t, reconciles, reconcile.Request{NamespacedName: types.NamespacedName{Name: "ns2"}})
	assert.Contains(t, reconciles, reconcile.Request{NamespacedName: types.NamespacedName{Name: "eclipse-che"}})
}

func TestWatchRulesForConfigMapsInOtherNamespaces(t *testing.T) {
	cm := &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ConfigMap",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      tls.CheAllCACertsConfigMapName,
			Namespace: "eclipse-che",
		},
	}

	_, _, r := setup(devworkspaceinfra.Kubernetes,
		&corev1.Namespace{
			TypeMeta: metav1.TypeMeta{
				Kind:       "Namespace",
				APIVersion: "v1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: "ns1",
				Labels: map[string]string{
					workspaceNamespaceOwnerUidLabel: "uid1",
				},
			},
		},
		&corev1.Namespace{
			TypeMeta: metav1.TypeMeta{
				Kind:       "Namespace",
				APIVersion: "v1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: "ns2",
				Labels: map[string]string{
					workspaceNamespaceOwnerUidLabel: "uid2",
				},
			},
		},
		&corev1.Namespace{
			TypeMeta: metav1.TypeMeta{
				Kind:       "Namespace",
				APIVersion: "v1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: "eclipse-che",
			},
		},
		&chev2.CheCluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "che",
				Namespace: "eclipse-che",
			},
		},
		cm)

	ctx := context.TODO()

	r.namespaceCache.ExamineNamespace(ctx, "ns1")
	r.namespaceCache.ExamineNamespace(ctx, "ns2")
	r.namespaceCache.ExamineNamespace(ctx, "eclipse-che")

	h := r.watchRulesForConfigMaps(ctx)
	rlq := workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter())
	// Let's throw event to controller about new config map creation.
	h.Create(event.CreateEvent{Object: cm}, rlq)

	amountReconcileRequests := rlq.Len()
	rs1, _ := rlq.Get()
	rs2, _ := rlq.Get()
	rs3, _ := rlq.Get()
	reconciles := []reconcile.Request{rs1.(reconcile.Request), rs2.(reconcile.Request), rs3.(reconcile.Request)}

	assert.Equal(t, 3, amountReconcileRequests)
	assert.Contains(t, reconciles, reconcile.Request{NamespacedName: types.NamespacedName{Name: "ns1"}})
	assert.Contains(t, reconciles, reconcile.Request{NamespacedName: types.NamespacedName{Name: "ns2"}})
	assert.Contains(t, reconciles, reconcile.Request{NamespacedName: types.NamespacedName{Name: "eclipse-che"}})
}
