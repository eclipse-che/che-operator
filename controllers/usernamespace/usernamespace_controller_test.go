//
// Copyright (c) 2019-2026 Red Hat, Inc.
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
	"testing"

	containerbuild "github.com/eclipse-che/che-operator/pkg/deploy/container-capabilities"
	"k8s.io/utils/ptr"

	rbacv1 "k8s.io/api/rbac/v1"

	dwconstants "github.com/devfile/devworkspace-operator/pkg/constants"
	chev2 "github.com/eclipse-che/che-operator/api/v2"
	"github.com/eclipse-che/che-operator/pkg/common/constants"
	"github.com/eclipse-che/che-operator/pkg/common/infrastructure"
	"github.com/eclipse-che/che-operator/pkg/deploy/tls"
	configv1 "github.com/openshift/api/config/v1"
	projectv1 "github.com/openshift/api/project/v1"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func TestSkipsUnlabeledNamespaces(t *testing.T) {
	test := func(t *testing.T, infraType infrastructure.Type, namespace metav1.Object) {
		ctx := context.TODO()
		_, cl, r := setup(infraType, namespace.(client.Object))
		setupCheCluster(t, ctx, cl, "che", "che")

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
		test(t, infrastructure.Kubernetes, &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: "ns",
			},
		})
	})

	t.Run("openshift", func(t *testing.T) {
		test(t, infrastructure.OpenShiftV4, &projectv1.Project{
			ObjectMeta: metav1.ObjectMeta{
				Name: "prj",
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

	test := func(t *testing.T, infraType infrastructure.Type, namespace client.Object, objs ...client.Object) {
		ctx := context.TODO()
		allObjs := append(objs, namespace)
		_, cl, r := setup(infraType, allObjs...)
		setupCheCluster(t, ctx, cl, "eclipse-che", "che")

		res, err := r.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: namespace.GetName()}})
		assert.NoError(t, err, "Reconciliation should have succeeded")

		assert.Empty(t, res.RequeueAfter)

		userSettings := corev1.ConfigMap{}
		assert.NoError(t, cl.Get(ctx, client.ObjectKey{Name: "che-user-settings", Namespace: namespace.GetName()}, &userSettings))

		assert.Equal(t, "env", userSettings.GetAnnotations()[dwconstants.DevWorkspaceMountAsAnnotation],
			"user settings should be annotated as mount as 'env'")

		assert.Equal(t, "true", userSettings.GetLabels()[dwconstants.DevWorkspaceMountLabel],
			"user settings should be labeled as mounted")

		assert.Equal(t, "1800", userSettings.Data["SECONDS_OF_DW_INACTIVITY_BEFORE_IDLING"], "Unexpected user settings")
		assert.Equal(t, "-1", userSettings.Data["SECONDS_OF_DW_RUN_BEFORE_IDLING"], "Unexpected user settings")

		assert.Equal(t, userSettings.Data["EDITOR_DOWNLOAD_URL_CHE_INCUBATOR_CHE_IDEA_LATEST"], "url_latest")
		assert.Equal(t, userSettings.Data["EDITOR_DOWNLOAD_URL_CHE_INCUBATOR_CHE_IDEA_NEXT"], "url_next")

		assert.Equal(t, "true", userSettings.Data["CLI_ACTIVITY_TRACKER_ENABLED"], "Unexpected CLI Activity Tracker enabled setting")
		assert.Equal(t, "30", userSettings.Data["CLI_ACTIVITY_TRACKER_CHECK_PERIOD"], "Unexpected CLI Activity Tracker check period")
		assert.Equal(t, "900", userSettings.Data["CLI_ACTIVITY_TRACKER_ACTIVITY_WINDOW"], "Unexpected CLI Activity Tracker activity window")
		assert.Equal(t, "300", userSettings.Data["CLI_ACTIVITY_TRACKER_GRACE_PERIOD"], "Unexpected CLI Activity Tracker grace period")
		_, hasMaxProcessAge := userSettings.Data["CLI_ACTIVITY_TRACKER_MAX_PROCESS_AGE"]
		assert.False(t, hasMaxProcessAge, "CLI_ACTIVITY_TRACKER_MAX_PROCESS_AGE should not be set when not configured (nil)")
		assert.Equal(t, "true", userSettings.Data["CLI_ACTIVITY_TRACKER_VERBOSE"], "Unexpected CLI Activity Tracker verbose setting")

		if infraType != infrastructure.Kubernetes {
			assert.Equal(t, userSettings.Data["NO_PROXY"], ".svc")
			assert.Equal(t, userSettings.Data["no_proxy"], ".svc")
		}

		cert := corev1.Secret{}
		assert.NoError(t, cl.Get(ctx, client.ObjectKey{Name: "che-server-cert", Namespace: namespace.GetName()}, &cert))

		assert.Equal(t, "file", cert.GetAnnotations()[dwconstants.DevWorkspaceMountAsAnnotation], "server cert should be annotated as mount as 'file'")
		assert.Equal(t, "/tmp/che/secret/", cert.GetAnnotations()[dwconstants.DevWorkspaceMountPathAnnotation], "server cert annotated as mounted to an unexpected path")
		assert.Equal(t, "true", cert.GetLabels()[dwconstants.DevWorkspaceMountLabel], "server cert should be labeled as mounted")
		assert.Equal(t, 1, len(cert.Data), "Expecting just 1 element in the self-signed cert")
		assert.Equal(t, "my certificate", string(cert.Data["ca.crt"]), "Unexpected self-signed certificate")
		assert.Equal(t, corev1.SecretTypeOpaque, cert.Type, "Unexpected secret type")
		assert.Equal(t, true, *cert.Immutable, "Unexpected mutability of the secret")

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
		test(t, infrastructure.Kubernetes, &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: "ns",
				Labels: map[string]string{
					constants.WorkspaceNamespaceOwnerUidLabelKey: "uid",
				},
			},
		})
	})

	t.Run("openshift", func(t *testing.T) {
		test(t, infrastructure.OpenShiftV4,
			&corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "prj",
					Labels: map[string]string{
						constants.WorkspaceNamespaceOwnerUidLabelKey: "uid",
					},
				},
			},
			&projectv1.Project{
				ObjectMeta: metav1.ObjectMeta{
					Name: "prj",
					Labels: map[string]string{
						constants.WorkspaceNamespaceOwnerUidLabelKey: "uid",
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

func TestOptionalFeaturesOmitEnvVars(t *testing.T) {
	infrastructure.InitializeForTesting(infrastructure.Kubernetes)

	testCases := []struct {
		name               string
		cliActivityTracker *chev2.CliActivityTrackerConfig
	}{
		{
			name:               "cliActivityTracker nil",
			cliActivityTracker: nil,
		},
		{
			name: "cliActivityTracker enabled explicitly false with timing fields set",
			cliActivityTracker: &chev2.CliActivityTrackerConfig{
				Enabled:                 ptr.To(false),
				SecondsOfCheckPeriod:    ptr.To(int32(30)),
				SecondsOfActivityWindow: ptr.To(int32(900)),
				SecondsOfGracePeriod:    ptr.To(int32(300)),
				SecondsOfMaxProcessAge:  ptr.To(int32(21600)),
				Verbose:                 ptr.To(true),
			},
		},
		{
			name: "cliActivityTracker enabled nil with timing fields set",
			cliActivityTracker: &chev2.CliActivityTrackerConfig{
				SecondsOfCheckPeriod:    ptr.To(int32(30)),
				SecondsOfActivityWindow: ptr.To(int32(900)),
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.TODO()
			namespace := &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "ns-" + tc.name,
					Labels: map[string]string{
						constants.WorkspaceNamespaceOwnerUidLabelKey: "uid",
					},
				},
			}
			scheme, cl, r := setup(infrastructure.Kubernetes, namespace)

			cheNamespace := &corev1.Namespace{}
			cheNamespace.SetName("eclipse-che")
			assert.NoError(t, cl.Create(ctx, cheNamespace))

			cheCluster := chev2.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "che",
					Namespace: "eclipse-che",
				},
				Spec: chev2.CheClusterSpec{
					DevEnvironments: chev2.CheClusterDevEnvironments{
						SecondsOfInactivityBeforeIdling: ptr.To(int32(1800)),
						SecondsOfRunBeforeIdling:        ptr.To(int32(-1)),
						CliActivityTracker:              tc.cliActivityTracker,
					},
				},
			}
			assert.NoError(t, cl.Create(ctx, &cheCluster))

			_ = scheme
			_, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: namespace.GetName()}})
			assert.NoError(t, err, "Reconciliation should have succeeded")

			userSettings := corev1.ConfigMap{}
			assert.NoError(t, cl.Get(ctx, client.ObjectKey{Name: "che-user-settings", Namespace: namespace.GetName()}, &userSettings))

			_, hasEnabled := userSettings.Data["CLI_ACTIVITY_TRACKER_ENABLED"]
			assert.False(t, hasEnabled, "CLI_ACTIVITY_TRACKER_ENABLED should not be set when tracker is not enabled")
			_, hasCheckPeriod := userSettings.Data["CLI_ACTIVITY_TRACKER_CHECK_PERIOD"]
			assert.False(t, hasCheckPeriod, "CLI_ACTIVITY_TRACKER_CHECK_PERIOD should not be set when tracker is not enabled")
			_, hasActivityWindow := userSettings.Data["CLI_ACTIVITY_TRACKER_ACTIVITY_WINDOW"]
			assert.False(t, hasActivityWindow, "CLI_ACTIVITY_TRACKER_ACTIVITY_WINDOW should not be set when tracker is not enabled")
			_, hasGracePeriod := userSettings.Data["CLI_ACTIVITY_TRACKER_GRACE_PERIOD"]
			assert.False(t, hasGracePeriod, "CLI_ACTIVITY_TRACKER_GRACE_PERIOD should not be set when tracker is not enabled")
			_, hasMaxProcessAge := userSettings.Data["CLI_ACTIVITY_TRACKER_MAX_PROCESS_AGE"]
			assert.False(t, hasMaxProcessAge, "CLI_ACTIVITY_TRACKER_MAX_PROCESS_AGE should not be set when tracker is not enabled")
			_, hasVerbose := userSettings.Data["CLI_ACTIVITY_TRACKER_VERBOSE"]
			assert.False(t, hasVerbose, "CLI_ACTIVITY_TRACKER_VERBOSE should not be set when tracker is not enabled")
		})
	}
}

func TestUpdateSccClusterRoleBinding(t *testing.T) {
	infrastructure.InitializeForTesting(infrastructure.OpenShiftV4)

	pr1 := &projectv1.Project{
		ObjectMeta: metav1.ObjectMeta{
			Name: "ns1",
			Labels: map[string]string{
				constants.WorkspaceNamespaceOwnerUidLabelKey: "uid_1",
			},
			Annotations: map[string]string{
				constants.CheEclipseOrgUsername: "user_1",
			},
		},
	}

	ns1 := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "ns1",
			Labels: map[string]string{
				constants.WorkspaceNamespaceOwnerUidLabelKey: "uid_1",
			},
			Annotations: map[string]string{
				constants.CheEclipseOrgUsername: "user_1",
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
				DisableContainerBuildCapabilities: ptr.To(false),
				ContainerBuildConfiguration: &chev2.ContainerBuildConfiguration{
					OpenShiftSecurityContextConstraint: "container-build",
				},
				DisableContainerRunCapabilities: ptr.To(false),
				ContainerRunConfiguration: &chev2.ContainerRunConfiguration{
					OpenShiftSecurityContextConstraint: "container-run",
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

	allObjs := []client.Object{ns1, pr1, cheCluster}
	_, cl, usernamespaceReconciler := setup(infrastructure.OpenShiftV4, allObjs...)

	_, err := usernamespaceReconciler.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: ns1.GetName()}})
	assert.Nil(t, err)

	rb := &rbacv1.RoleBinding{}
	err = cl.Get(context.TODO(), types.NamespacedName{Name: containerbuild.NewContainerBuild().GetUserRoleName(), Namespace: "ns1"}, rb)
	assert.Nil(t, err)
	assert.Equal(t, "user_1", rb.Subjects[0].Name)

	rb = &rbacv1.RoleBinding{}
	err = cl.Get(context.TODO(), types.NamespacedName{Name: containerbuild.NewContainerRun().GetUserRoleName(), Namespace: "ns1"}, rb)
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

	_, _, r := setup(infrastructure.Kubernetes, &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "ns",
			Labels: map[string]string{
				constants.WorkspaceNamespaceOwnerUidLabelKey: "uid",
			},
		},
	}, secret)

	ctx := context.TODO()

	h := r.watchRulesForSecrets(ctx)
	rlq := workqueue.NewTypedRateLimitingQueue(workqueue.DefaultTypedControllerRateLimiter[reconcile.Request]())
	// Let's throw event to controller about new secret creation.
	h.Create(context.TODO(), event.CreateEvent{Object: secret}, rlq)

	amountReconcileRequests := rlq.Len()
	rs, _ := rlq.Get()

	assert.Equal(t, 1, amountReconcileRequests)
	assert.Equal(t, "ns", rs.Name)
}

func TestWatchRulesForConfigMapsInSameNamespace(t *testing.T) {
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "cm",
			Namespace: "ns",
			Labels:    map[string]string{"app.kubernetes.io/component": "user-settings"},
		},
	}

	_, _, r := setup(infrastructure.Kubernetes, &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "ns",
			Labels: map[string]string{
				constants.WorkspaceNamespaceOwnerUidLabelKey: "uid",
			},
		},
	}, cm)

	ctx := context.TODO()

	h := r.watchRulesForSecrets(ctx)
	rlq := workqueue.NewTypedRateLimitingQueue(workqueue.DefaultTypedControllerRateLimiter[reconcile.Request]())
	// Let's throw event to controller about new config map creation.
	h.Create(context.TODO(), event.CreateEvent{Object: cm}, rlq)

	amountReconcileRequests := rlq.Len()
	rs, _ := rlq.Get()

	assert.Equal(t, 1, amountReconcileRequests)
	assert.Equal(t, "ns", rs.Name)
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

	_, _, r := setup(infrastructure.Kubernetes,
		&corev1.Namespace{
			TypeMeta: metav1.TypeMeta{
				Kind:       "Namespace",
				APIVersion: "v1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: "ns1",
				Labels: map[string]string{
					constants.WorkspaceNamespaceOwnerUidLabelKey: "uid1",
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
					constants.WorkspaceNamespaceOwnerUidLabelKey: "uid2",
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

	_, _ = r.namespaceCache.ExamineNamespace(ctx, "ns1")
	_, _ = r.namespaceCache.ExamineNamespace(ctx, "ns2")
	_, _ = r.namespaceCache.ExamineNamespace(ctx, "eclipse-che")

	h := r.watchRulesForSecrets(ctx)
	rlq := workqueue.NewTypedRateLimitingQueue(workqueue.DefaultTypedControllerRateLimiter[reconcile.Request]())
	// Let's throw event to controller about new secret creation.
	h.Create(context.TODO(), event.CreateEvent{Object: secret}, rlq)

	amountReconcileRequests := rlq.Len()
	rs1, _ := rlq.Get()
	rs2, _ := rlq.Get()
	rs3, _ := rlq.Get()
	reconciles := []reconcile.Request{rs1, rs2, rs3}

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
			Name:      tls.CheMergedCABundleCertsCMName,
			Namespace: "eclipse-che",
		},
	}

	_, _, r := setup(infrastructure.Kubernetes,
		&corev1.Namespace{
			TypeMeta: metav1.TypeMeta{
				Kind:       "Namespace",
				APIVersion: "v1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: "ns1",
				Labels: map[string]string{
					constants.WorkspaceNamespaceOwnerUidLabelKey: "uid1",
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
					constants.WorkspaceNamespaceOwnerUidLabelKey: "uid2",
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

	_, _ = r.namespaceCache.ExamineNamespace(ctx, "ns1")
	_, _ = r.namespaceCache.ExamineNamespace(ctx, "ns2")
	_, _ = r.namespaceCache.ExamineNamespace(ctx, "eclipse-che")

	h := r.watchRulesForConfigMaps(ctx)
	rlq := workqueue.NewTypedRateLimitingQueue(workqueue.DefaultTypedControllerRateLimiter[reconcile.Request]())
	// Let's throw event to controller about new config map creation.
	h.Create(context.TODO(), event.CreateEvent{Object: cm}, rlq)

	amountReconcileRequests := rlq.Len()
	rs1, _ := rlq.Get()
	rs2, _ := rlq.Get()
	rs3, _ := rlq.Get()
	reconciles := []reconcile.Request{rs1, rs2, rs3}

	assert.Equal(t, 3, amountReconcileRequests)
	assert.Contains(t, reconciles, reconcile.Request{NamespacedName: types.NamespacedName{Name: "ns1"}})
	assert.Contains(t, reconciles, reconcile.Request{NamespacedName: types.NamespacedName{Name: "ns2"}})
	assert.Contains(t, reconciles, reconcile.Request{NamespacedName: types.NamespacedName{Name: "eclipse-che"}})
}
