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

package openvsx_server

import (
	"testing"

	chev2 "github.com/eclipse-che/che-operator/api/v2"
	"github.com/eclipse-che/che-operator/pkg/common/chetypes"
	"github.com/eclipse-che/che-operator/pkg/common/constants"
	defaults "github.com/eclipse-che/che-operator/pkg/common/operator-defaults"
	"github.com/eclipse-che/che-operator/pkg/common/test"
	"github.com/eclipse-che/che-operator/pkg/deploy/openvsx"
	"github.com/stretchr/testify/assert"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func reconcileWithReadyDeployment(
	reconciler *OpenVSXServerReconciler,
) func(cheCtx *chetypes.CheContext) (reconcile.Result, bool, error) {
	return func(cheCtx *chetypes.CheContext) (reconcile.Result, bool, error) {
		result, done, err := reconciler.Reconcile(cheCtx)
		if !done && err == nil {
			deploymentKey := types.NamespacedName{
				Name:      constants.OpenVSXServerComponentName,
				Namespace: cheCtx.CheCluster.Namespace,
			}

			deployment := &appsv1.Deployment{}
			exists, _ := cheCtx.ClusterAPI.ClientWrapper.GetIgnoreNotFound(cheCtx.Context, deploymentKey, deployment)
			if exists {
				deployment.Status.AvailableReplicas = 1
				deployment.Status.UnavailableReplicas = 0
				_ = cheCtx.ClusterAPI.Client.Status().Update(cheCtx.Context, deployment)
			}
		}

		return result, done, err
	}
}

func cronJobKey(cheCtx *chetypes.CheContext) types.NamespacedName {
	return types.NamespacedName{
		Name:      constants.OpenVSXServerExtensionUpdateCronJobName,
		Namespace: cheCtx.CheCluster.Namespace,
	}
}

func TestExtensionAutoUpdateCronJobCreated(t *testing.T) {
	cheCtx := test.NewCtxBuilder().WithCheCluster(
		&chev2.CheCluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "eclipse-che",
				Namespace: "eclipse-che",
			},
			Spec: chev2.CheClusterSpec{
				Components: chev2.CheClusterComponents{
					OpenVSXRegistry: chev2.OpenVSXRegistry{
						Enable: true,
						ExtensionAutoUpdate: &chev2.ExtensionAutoUpdate{
							Enable: ptr.To(true),
						},
					},
				},
			},
		},
	).Build()

	reconciler := NewOpenVSXServerReconciler()
	test.EnsureReconcile(t, cheCtx, reconcileWithReadyDeployment(reconciler))

	assert.True(t,
		test.IsObjectExists(cheCtx.ClusterAPI.Client, cronJobKey(cheCtx), &batchv1.CronJob{}),
		"CronJob should be created when auto-update is enabled",
	)
}

func TestExtensionAutoUpdateCronJobNotCreatedWhenDisabled(t *testing.T) {
	cheCtx := test.NewCtxBuilder().WithCheCluster(
		&chev2.CheCluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "eclipse-che",
				Namespace: "eclipse-che",
			},
			Spec: chev2.CheClusterSpec{
				Components: chev2.CheClusterComponents{
					OpenVSXRegistry: chev2.OpenVSXRegistry{
						Enable: true,
					},
				},
			},
		},
	).Build()

	reconciler := NewOpenVSXServerReconciler()
	test.EnsureReconcile(t, cheCtx, reconcileWithReadyDeployment(reconciler))

	assert.False(t,
		test.IsObjectExists(cheCtx.ClusterAPI.Client, cronJobKey(cheCtx), &batchv1.CronJob{}),
		"CronJob should not be created when auto-update is not configured",
	)
}

func TestExtensionAutoUpdateCronJobCleanedUpOnDisable(t *testing.T) {
	cheCtx := test.NewCtxBuilder().WithCheCluster(
		&chev2.CheCluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "eclipse-che",
				Namespace: "eclipse-che",
			},
			Spec: chev2.CheClusterSpec{
				Components: chev2.CheClusterComponents{
					OpenVSXRegistry: chev2.OpenVSXRegistry{
						Enable: true,
						ExtensionAutoUpdate: &chev2.ExtensionAutoUpdate{
							Enable: ptr.To(true),
						},
					},
				},
			},
		},
	).Build()

	reconciler := NewOpenVSXServerReconciler()
	test.EnsureReconcile(t, cheCtx, reconcileWithReadyDeployment(reconciler))

	assert.True(t, test.IsObjectExists(cheCtx.ClusterAPI.Client, cronJobKey(cheCtx), &batchv1.CronJob{}))

	cheCtx.CheCluster.Spec.Components.OpenVSXRegistry.ExtensionAutoUpdate.Enable = ptr.To(false)
	test.EnsureReconcile(t, cheCtx, reconcileWithReadyDeployment(reconciler))

	assert.False(t,
		test.IsObjectExists(cheCtx.ClusterAPI.Client, cronJobKey(cheCtx), &batchv1.CronJob{}),
		"CronJob should be deleted when auto-update is disabled",
	)
}

func TestExtensionAutoUpdateCronJobSpec(t *testing.T) {
	customSchedule := "0 */6 * * *"
	engineVersion := "1.92.0"
	excludeExtensions := []string{"redhat.java", "redhat.vscode-xml"}

	cheCtx := test.NewCtxBuilder().WithCheCluster(
		&chev2.CheCluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "eclipse-che",
				Namespace: "eclipse-che",
			},
			Spec: chev2.CheClusterSpec{
				Components: chev2.CheClusterComponents{
					OpenVSXRegistry: chev2.OpenVSXRegistry{
						Enable: true,
						ExtensionAutoUpdate: &chev2.ExtensionAutoUpdate{
							Enable:              ptr.To(true),
							Schedule:            ptr.To(customSchedule),
							VSCodeEngineVersion: ptr.To(engineVersion),
							ExcludedExtensions:  excludeExtensions,
						},
					},
				},
			},
			Status: chev2.CheClusterStatus{
				CheURL: "https://eclipse-che.apps.example.com",
			},
		},
	).Build()

	reconciler := NewOpenVSXServerReconciler()
	cronJob, err := reconciler.getExtensionUpdateCronJobSpec(cheCtx)
	if !assert.NoError(t, err) {
		return
	}

	assert.Equal(t, customSchedule, cronJob.Spec.Schedule)
	assert.Equal(t, batchv1.ForbidConcurrent, cronJob.Spec.ConcurrencyPolicy)

	container := cronJob.Spec.JobTemplate.Spec.Template.Spec.Containers[0]
	assert.Equal(t, defaults.GetOpenVSXImage(cheCtx.CheCluster), container.Image)

	envMap := make(map[string]string)
	for _, e := range container.Env {
		envMap[e.Name] = e.Value
	}

	assert.Equal(t, openvsx.GetOpenVSXServerServiceURL(cheCtx), envMap["OVSX_REGISTRY_URL"])
	assert.Equal(t, engineVersion, envMap["VSCODE_ENGINE_VERSION"])
	assert.Equal(t, "redhat.java,redhat.vscode-xml", envMap["EXCLUDE_EXTENSIONS"])
	assert.Equal(t, "eclipse-che.apps.example.com", envMap["OVSX_FORWARDED_HOST"])

	patEnv := findEnvVar(container.Env, "OVSX_PAT")
	if assert.NotNil(t, patEnv, "OVSX_PAT env var should be present") {
		assert.NotNil(t, patEnv.ValueFrom)
		assert.NotNil(t, patEnv.ValueFrom.SecretKeyRef)
		assert.Equal(t, openvsx.GetCredentialsSecretName(cheCtx), patEnv.ValueFrom.SecretKeyRef.Name)
		assert.Equal(t, "openvsx-publisher-token", patEnv.ValueFrom.SecretKeyRef.Key)
	}
}

func TestExtensionAutoUpdateCronJobNoExcludedExtensions(t *testing.T) {
	cheCtx := test.NewCtxBuilder().WithCheCluster(
		&chev2.CheCluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "eclipse-che",
				Namespace: "eclipse-che",
			},
			Spec: chev2.CheClusterSpec{
				Components: chev2.CheClusterComponents{
					OpenVSXRegistry: chev2.OpenVSXRegistry{
						Enable: true,
						ExtensionAutoUpdate: &chev2.ExtensionAutoUpdate{
							Enable: ptr.To(true),
						},
					},
				},
			},
		},
	).Build()

	reconciler := NewOpenVSXServerReconciler()
	cronJob, err := reconciler.getExtensionUpdateCronJobSpec(cheCtx)
	if !assert.NoError(t, err) {
		return
	}

	container := cronJob.Spec.JobTemplate.Spec.Template.Spec.Containers[0]
	envNames := envVarNames(container.Env)
	assert.NotContains(t,
		envNames,
		"EXCLUDE_EXTENSIONS",
		"EXCLUDE_EXTENSIONS should not be set when excludeExtensions is empty",
	)
}

func TestExtensionAutoUpdateCronJobDefaultSchedule(t *testing.T) {
	cheCtx := test.NewCtxBuilder().WithCheCluster(
		&chev2.CheCluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "eclipse-che",
				Namespace: "eclipse-che",
			},
			Spec: chev2.CheClusterSpec{
				Components: chev2.CheClusterComponents{
					OpenVSXRegistry: chev2.OpenVSXRegistry{
						Enable: true,
						ExtensionAutoUpdate: &chev2.ExtensionAutoUpdate{
							Enable: ptr.To(true),
						},
					},
				},
			},
		},
	).Build()

	reconciler := NewOpenVSXServerReconciler()
	cronJob, err := reconciler.getExtensionUpdateCronJobSpec(cheCtx)
	if !assert.NoError(t, err) {
		return
	}

	assert.Equal(t, constants.DefaultExtensionAutoUpdateSchedule, cronJob.Spec.Schedule)
}

func TestExtensionAutoUpdateCronJobNoEngineVersion(t *testing.T) {
	cheCtx := test.NewCtxBuilder().WithCheCluster(
		&chev2.CheCluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "eclipse-che",
				Namespace: "eclipse-che",
			},
			Spec: chev2.CheClusterSpec{
				Components: chev2.CheClusterComponents{
					OpenVSXRegistry: chev2.OpenVSXRegistry{
						Enable: true,
						ExtensionAutoUpdate: &chev2.ExtensionAutoUpdate{
							Enable: ptr.To(true),
						},
					},
				},
			},
		},
	).Build()

	reconciler := NewOpenVSXServerReconciler()
	cronJob, err := reconciler.getExtensionUpdateCronJobSpec(cheCtx)
	if !assert.NoError(t, err) {
		return
	}

	container := cronJob.Spec.JobTemplate.Spec.Template.Spec.Containers[0]
	envNames := envVarNames(container.Env)
	assert.NotContains(t,
		envNames,
		"VSCODE_ENGINE_VERSION",
		"VSCODE_ENGINE_VERSION should not be set when vsCodeEngineVersion is omitted",
	)
}

func TestExtensionAutoUpdateCronJobCleanedUpWhenRegistryDisabled(t *testing.T) {
	cheCtx := test.NewCtxBuilder().WithCheCluster(
		&chev2.CheCluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "eclipse-che",
				Namespace: "eclipse-che",
			},
			Spec: chev2.CheClusterSpec{
				Components: chev2.CheClusterComponents{
					OpenVSXRegistry: chev2.OpenVSXRegistry{
						Enable: true,
						ExtensionAutoUpdate: &chev2.ExtensionAutoUpdate{
							Enable: ptr.To(true),
						},
					},
				},
			},
		},
	).Build()

	reconciler := NewOpenVSXServerReconciler()
	test.EnsureReconcile(t, cheCtx, reconcileWithReadyDeployment(reconciler))

	cronJob := &batchv1.CronJob{}
	exists, err := cheCtx.ClusterAPI.ClientWrapper.GetIgnoreNotFound(cheCtx.Context, cronJobKey(cheCtx), cronJob)
	assert.NoError(t, err)
	assert.True(t, exists, "CronJob should exist")

	cheCtx.CheCluster.Spec.Components.OpenVSXRegistry.Enable = false
	test.EnsureReconcile(t, cheCtx, reconcileWithReadyDeployment(reconciler))

	assert.False(t,
		test.IsObjectExists(cheCtx.ClusterAPI.Client, cronJobKey(cheCtx), &batchv1.CronJob{}),
		"CronJob should be deleted when OpenVSX registry is disabled",
	)
}

func findEnvVar(envVars []corev1.EnvVar, name string) *corev1.EnvVar {
	for i := range envVars {
		if envVars[i].Name == name {
			return &envVars[i]
		}
	}
	return nil
}

func envVarNames(envVars []corev1.EnvVar) []string {
	names := make([]string, len(envVars))
	for i, e := range envVars {
		names[i] = e.Name
	}
	return names
}
