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
	"context"
	"testing"

	chev2 "github.com/eclipse-che/che-operator/api/v2"
	"github.com/eclipse-che/che-operator/pkg/common/chetypes"
	"github.com/eclipse-che/che-operator/pkg/common/constants"
	defaults "github.com/eclipse-che/che-operator/pkg/common/operator-defaults"
	"github.com/eclipse-che/che-operator/pkg/common/test"
	"github.com/eclipse-che/che-operator/pkg/deploy"
	"github.com/eclipse-che/che-operator/pkg/deploy/openvsx"
	"github.com/stretchr/testify/assert"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func reconcileWithReadyDeployment(
	reconciler *OpenVSXServerReconciler,
) func(ctx *chetypes.DeployContext) (reconcile.Result, bool, error) {
	return func(ctx *chetypes.DeployContext) (reconcile.Result, bool, error) {
		result, done, err := reconciler.Reconcile(ctx)
		if !done && err == nil {
			deployment := &appsv1.Deployment{}
			if exists, _ := deploy.GetNamespacedObject(ctx, constants.OpenVSXServerComponentName, deployment); exists {
				deployment.Status.AvailableReplicas = 1
				deployment.Status.UnavailableReplicas = 0
				_ = ctx.ClusterAPI.Client.Status().Update(context.TODO(), deployment)
			}
		}

		return result, done, err
	}
}

func TestExtensionAutoUpdateCronJobCreated(t *testing.T) {
	ctx := test.NewCtxBuilder().WithCheCluster(
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
							Enable: true,
						},
					},
				},
			},
		},
	).Build()

	reconciler := NewOpenVSXServerReconciler()
	test.EnsureReconcile(t, ctx, reconcileWithReadyDeployment(reconciler))

	ns := "eclipse-che"
	assert.True(t,
		test.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: constants.OpenVSXServerExtensionUpdateCronJobName, Namespace: ns}, &batchv1.CronJob{}),
		"CronJob should be created when auto-update is enabled",
	)
}

func TestExtensionAutoUpdateCronJobNotCreatedWhenDisabled(t *testing.T) {
	ctx := test.NewCtxBuilder().WithCheCluster(
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
	test.EnsureReconcile(t, ctx, reconcileWithReadyDeployment(reconciler))

	ns := "eclipse-che"
	assert.False(t,
		test.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: constants.OpenVSXServerExtensionUpdateCronJobName, Namespace: ns}, &batchv1.CronJob{}),
		"CronJob should not be created when auto-update is not configured",
	)
}

func TestExtensionAutoUpdateCronJobCleanedUpOnDisable(t *testing.T) {
	ctx := test.NewCtxBuilder().WithCheCluster(
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
							Enable: true,
						},
					},
				},
			},
		},
	).Build()

	reconciler := NewOpenVSXServerReconciler()
	test.EnsureReconcile(t, ctx, reconcileWithReadyDeployment(reconciler))

	ns := "eclipse-che"
	assert.True(t, test.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: constants.OpenVSXServerExtensionUpdateCronJobName, Namespace: ns}, &batchv1.CronJob{}))

	ctx.CheCluster.Spec.Components.OpenVSXRegistry.ExtensionAutoUpdate.Enable = false
	test.EnsureReconcile(t, ctx, reconcileWithReadyDeployment(reconciler))

	assert.False(t,
		test.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: constants.OpenVSXServerExtensionUpdateCronJobName, Namespace: ns}, &batchv1.CronJob{}),
		"CronJob should be deleted when auto-update is disabled",
	)
}

func TestExtensionAutoUpdateCronJobSpec(t *testing.T) {
	customSchedule := "0 */6 * * *"
	engineVersion := "1.92.0"
	excludeExtensions := []string{"redhat/java", "redhat/vscode-xml"}

	ctx := test.NewCtxBuilder().WithCheCluster(
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
							Enable:              true,
							Schedule:            ptr.To(customSchedule),
							VSCodeEngineVersion: ptr.To(engineVersion),
							ExcludeExtensions:   excludeExtensions,
						},
					},
				},
			},
		},
	).Build()

	reconciler := NewOpenVSXServerReconciler()
	cronJob, err := reconciler.getExtensionUpdateCronJobSpec(ctx)
	if !assert.NoError(t, err) {
		return
	}

	assert.Equal(t, customSchedule, cronJob.Spec.Schedule)
	assert.Equal(t, batchv1.ForbidConcurrent, cronJob.Spec.ConcurrencyPolicy)
	assert.Equal(t, defaults.GetOpenVSXImage(ctx.CheCluster), cronJob.Spec.JobTemplate.Spec.Template.Spec.Containers[0].Image)

	envMap := make(map[string]string)
	for _, e := range cronJob.Spec.JobTemplate.Spec.Template.Spec.Containers[0].Env {
		if e.Value != "" {
			envMap[e.Name] = e.Value
		}
	}

	assert.Equal(t, openvsx.GetOpenVSXServerServiceURL(ctx), envMap["OVSX_REGISTRY_URL"])
	assert.Equal(t, engineVersion, envMap["VSCODE_ENGINE_VERSION"])
	assert.Equal(t, "redhat/java,redhat/vscode-xml", envMap["EXCLUDE_EXTENSIONS"])
}

func TestExtensionAutoUpdateCronJobNoExcludeExtensions(t *testing.T) {
	ctx := test.NewCtxBuilder().WithCheCluster(
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
							Enable: true,
						},
					},
				},
			},
		},
	).Build()

	reconciler := NewOpenVSXServerReconciler()
	cronJob, err := reconciler.getExtensionUpdateCronJobSpec(ctx)
	if !assert.NoError(t, err) {
		return
	}

	container := cronJob.Spec.JobTemplate.Spec.Template.Spec.Containers[0]
	for _, e := range container.Env {
		assert.NotEqual(t, "EXCLUDE_EXTENSIONS", e.Name, "EXCLUDE_EXTENSIONS should not be set when excludeExtensions is empty")
	}
}

func TestExtensionAutoUpdateCronJobDefaultSchedule(t *testing.T) {
	ctx := test.NewCtxBuilder().WithCheCluster(
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
							Enable: true,
						},
					},
				},
			},
		},
	).Build()

	reconciler := NewOpenVSXServerReconciler()
	cronJob, err := reconciler.getExtensionUpdateCronJobSpec(ctx)
	if !assert.NoError(t, err) {
		return
	}

	assert.Equal(t, constants.DefaultExtensionAutoUpdateSchedule, cronJob.Spec.Schedule)
}

func TestExtensionAutoUpdateCronJobNoEngineVersion(t *testing.T) {
	ctx := test.NewCtxBuilder().WithCheCluster(
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
							Enable: true,
						},
					},
				},
			},
		},
	).Build()

	reconciler := NewOpenVSXServerReconciler()
	cronJob, err := reconciler.getExtensionUpdateCronJobSpec(ctx)
	if !assert.NoError(t, err) {
		return
	}

	container := cronJob.Spec.JobTemplate.Spec.Template.Spec.Containers[0]
	for _, e := range container.Env {
		assert.NotEqual(t, "VSCODE_ENGINE_VERSION", e.Name, "VSCODE_ENGINE_VERSION should not be set when vsCodeEngineVersion is omitted")
	}
}

func TestExtensionAutoUpdateCronJobCleanedUpWhenRegistryDisabled(t *testing.T) {
	ctx := test.NewCtxBuilder().WithCheCluster(
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
							Enable: true,
						},
					},
				},
			},
		},
	).Build()

	reconciler := NewOpenVSXServerReconciler()
	test.EnsureReconcile(t, ctx, reconcileWithReadyDeployment(reconciler))

	ns := "eclipse-che"
	cronJobKey := types.NamespacedName{Name: constants.OpenVSXServerExtensionUpdateCronJobName, Namespace: ns}

	cronJob := &batchv1.CronJob{}
	err := ctx.ClusterAPI.Client.Get(context.TODO(), cronJobKey, cronJob)
	assert.NoError(t, err, "CronJob should exist")

	ctx.CheCluster.Spec.Components.OpenVSXRegistry.Enable = false
	test.EnsureReconcile(t, ctx, reconcileWithReadyDeployment(reconciler))

	assert.False(t,
		test.IsObjectExists(ctx.ClusterAPI.Client, cronJobKey, &batchv1.CronJob{}),
		"CronJob should be deleted when OpenVSX registry is disabled",
	)
}
