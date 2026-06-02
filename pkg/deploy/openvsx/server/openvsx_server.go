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

package server

import (
	"context"
	"strings"

	"github.com/eclipse-che/che-operator/pkg/common/chetypes"
	"github.com/eclipse-che/che-operator/pkg/common/constants"
	"github.com/eclipse-che/che-operator/pkg/common/diffs"
	defaults "github.com/eclipse-che/che-operator/pkg/common/operator-defaults"
	"github.com/eclipse-che/che-operator/pkg/common/reconciler"
	"github.com/eclipse-che/che-operator/pkg/common/utils"
	"github.com/eclipse-che/che-operator/pkg/deploy"
	"github.com/eclipse-che/che-operator/pkg/deploy/tls"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type OpenVSXServerReconciler struct {
	reconciler.Reconcilable
}

func NewOpenVSXServerReconciler() *OpenVSXServerReconciler {
	return &OpenVSXServerReconciler{}
}

const (
	userSetupJobName        = "openvsx-user-setup"
	extensionsConfigMapName = "openvsx-extensions"
	extensionPublishJobName = "openvsx-extension-publish"
)

func (r *OpenVSXServerReconciler) Reconcile(ctx *chetypes.DeployContext) (reconcile.Result, bool, error) {
	if !ctx.CheCluster.IsOpenVSXOperandEnabled() {
		_, _ = deploy.DeleteNamespacedObject(ctx, constants.OpenVSXServerName, &appsv1.Deployment{})
		_, _ = deploy.DeleteNamespacedObject(ctx, constants.OpenVSXServerName, &corev1.Service{})
		_, _ = deploy.DeleteNamespacedObject(ctx, configMapName, &corev1.ConfigMap{})
		_, _ = deploy.DeleteNamespacedObject(ctx, userSetupJobName, &batchv1.Job{})
		_, _ = deploy.DeleteNamespacedObject(ctx, extensionPublishJobName, &batchv1.Job{})
		_, _ = deploy.DeleteNamespacedObject(ctx, extensionsConfigMapName, &corev1.ConfigMap{})
		return reconcile.Result{}, true, nil
	}

	done, err := r.syncService(ctx)
	if !done {
		return reconcile.Result{}, false, err
	}

	done, err = r.syncConfigMap(ctx)
	if !done {
		return reconcile.Result{}, false, err
	}

	done, err = r.syncDeployment(ctx)
	if !done {
		return reconcile.Result{}, false, err
	}

	done, err = r.syncUserSetupJob(ctx)
	if !done {
		return reconcile.Result{}, false, err
	}

	done, err = r.syncExtensionsConfigMap(ctx)
	if !done {
		return reconcile.Result{}, false, err
	}

	done, err = r.syncExtensionPublishJob(ctx)
	if !done {
		return reconcile.Result{}, false, err
	}

	return reconcile.Result{}, true, nil
}

func (r *OpenVSXServerReconciler) Finalize(ctx *chetypes.DeployContext) bool {
	return true
}

func (r *OpenVSXServerReconciler) syncService(ctx *chetypes.DeployContext) (bool, error) {
	return deploy.SyncServiceToCluster(
		ctx,
		constants.OpenVSXServerName,
		[]string{"http"},
		[]int32{8080},
		constants.OpenVSXServerName)
}

func (r *OpenVSXServerReconciler) syncConfigMap(ctx *chetypes.DeployContext) (bool, error) {
	cm := &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ConfigMap",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      configMapName,
			Namespace: ctx.CheCluster.Namespace,
			Labels:    deploy.GetLabels(constants.OpenVSXServerName),
		},
		Data: map[string]string{
			"application.yml": applicationConfig,
		},
	}

	return deploy.Sync(ctx, cm, diffs.ConfigMapAllLabels)
}

func (r *OpenVSXServerReconciler) syncDeployment(ctx *chetypes.DeployContext) (bool, error) {
	spec, err := r.getDeploymentSpec(ctx)
	if err != nil {
		return false, err
	}

	return deploy.SyncDeploymentSpecToCluster(ctx, spec, deploy.DefaultDeploymentDiffOpts)
}

func (r *OpenVSXServerReconciler) syncUserSetupJob(ctx *chetypes.DeployContext) (bool, error) {
	image := defaults.GetOpenVSXPostgresImage(ctx.CheCluster)
	pullPolicy := corev1.PullPolicy(utils.GetPullPolicyFromDockerImage(image))
	labels := deploy.GetLabels(constants.OpenVSXServerName)
	backoffLimit := int32(3)
	parallelism := int32(1)
	completions := int32(1)
	terminationGracePeriodSeconds := int64(30)
	runAsNonRoot := true

	secretName := constants.OpenVSXPostgresCredentialsSecret

	dbEnvVars := []corev1.EnvVar{
		{
			Name:  "PGHOST",
			Value: constants.OpenVSXPostgresName,
		},
		envFromSecret("PGDATABASE", secretName, "database"),
		envFromSecret("PGUSER", secretName, "user"),
		envFromSecret("PGPASSWORD", secretName, "password"),
	}

	job := &batchv1.Job{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Job",
			APIVersion: batchv1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      userSetupJobName,
			Namespace: ctx.CheCluster.Namespace,
			Labels:    labels,
		},
		Spec: batchv1.JobSpec{
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					RestartPolicy:                 corev1.RestartPolicyNever,
					TerminationGracePeriodSeconds: &terminationGracePeriodSeconds,
					SecurityContext: &corev1.PodSecurityContext{
						RunAsNonRoot: &runAsNonRoot,
					},
					InitContainers: []corev1.Container{
						{
							Name:            "wait-for-schema",
							Image:           image,
							ImagePullPolicy: pullPolicy,
							Env:             dbEnvVars,
							Command: []string{"sh", "-c",
								`until psql -c "SELECT 1 FROM user_data LIMIT 0" 2>/dev/null; do echo "Waiting for Flyway migrations..."; sleep 5; done`,
							},
						},
					},
					Containers: []corev1.Container{
						{
							Name:            userSetupJobName,
							Image:           image,
							ImagePullPolicy: pullPolicy,
							Env: append(dbEnvVars,
								envFromSecret("OPENVSX_USER_NAME", secretName, "userName"),
								envFromSecret("OPENVSX_USER_PAT", secretName, "userPAT"),
								envFromSecret("OPENVSX_ADMIN_NAME", secretName, "adminName"),
								envFromSecret("OPENVSX_ADMIN_PAT", secretName, "adminPAT"),
							),
							Command: []string{"sh", "-c",
								`psql -c "INSERT INTO user_data (id, login_name, role) VALUES (1001, '$OPENVSX_USER_NAME', 'privileged') ON CONFLICT (id) DO NOTHING; INSERT INTO personal_access_token (id, user_data, value, active, created_timestamp, accessed_timestamp, description, notified) VALUES (1001, 1001, '$OPENVSX_USER_PAT', true, current_timestamp, current_timestamp, 'extensions publisher', false) ON CONFLICT (id) DO NOTHING; INSERT INTO user_data (id, login_name, role) VALUES (1002, '$OPENVSX_ADMIN_NAME', 'admin') ON CONFLICT (id) DO NOTHING; INSERT INTO personal_access_token (id, user_data, value, active, created_timestamp, accessed_timestamp, description, notified) VALUES (1002, 1002, '$OPENVSX_ADMIN_PAT', true, current_timestamp, current_timestamp, 'Admin API Token', false) ON CONFLICT (id) DO NOTHING;"`,
							},
						},
					},
				},
			},
			Parallelism:  &parallelism,
			BackoffLimit: &backoffLimit,
			Completions:  &completions,
		},
	}

	return deploy.Sync(ctx, job, deploy.JobDiffOpts)
}

func (r *OpenVSXServerReconciler) syncExtensionsConfigMap(ctx *chetypes.DeployContext) (bool, error) {
	existing := &corev1.ConfigMap{}
	err := ctx.ClusterAPI.Client.Get(context.TODO(), types.NamespacedName{
		Name:      extensionsConfigMapName,
		Namespace: ctx.CheCluster.Namespace,
	}, existing)

	if err == nil {
		return true, nil
	}

	if !errors.IsNotFound(err) {
		return false, err
	}

	cm := &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ConfigMap",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      extensionsConfigMapName,
			Namespace: ctx.CheCluster.Namespace,
			Labels:    deploy.GetLabels(constants.OpenVSXServerName),
		},
		Data: map[string]string{
			"extensions.txt": "",
		},
	}

	return deploy.Sync(ctx, cm, diffs.ConfigMapAllLabels)
}

func (r *OpenVSXServerReconciler) syncExtensionPublishJob(ctx *chetypes.DeployContext) (bool, error) {
	cm := &corev1.ConfigMap{}
	err := ctx.ClusterAPI.Client.Get(context.TODO(), types.NamespacedName{
		Name:      extensionsConfigMapName,
		Namespace: ctx.CheCluster.Namespace,
	}, cm)
	if err != nil {
		return false, err
	}

	extensionsList := strings.TrimSpace(cm.Data["extensions.txt"])
	if extensionsList == "" {
		_, _ = deploy.DeleteNamespacedObject(ctx, extensionPublishJobName, &batchv1.Job{})
		return true, nil
	}

	image := defaults.GetOpenVSXImage(ctx.CheCluster)
	pullPolicy := corev1.PullPolicy(utils.GetPullPolicyFromDockerImage(image))
	labels := deploy.GetLabels(constants.OpenVSXServerName)
	backoffLimit := int32(3)
	parallelism := int32(1)
	completions := int32(1)
	terminationGracePeriodSeconds := int64(30)
	runAsNonRoot := true

	secretName := constants.OpenVSXPostgresCredentialsSecret
	extensionsHash := utils.ComputeHash256([]byte(extensionsList))

	job := &batchv1.Job{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Job",
			APIVersion: batchv1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      extensionPublishJobName,
			Namespace: ctx.CheCluster.Namespace,
			Labels:    labels,
		},
		Spec: batchv1.JobSpec{
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					RestartPolicy:                 corev1.RestartPolicyNever,
					TerminationGracePeriodSeconds: &terminationGracePeriodSeconds,
					SecurityContext: &corev1.PodSecurityContext{
						RunAsNonRoot: &runAsNonRoot,
					},
					Containers: []corev1.Container{
						{
							Name:            extensionPublishJobName,
							Image:           image,
							ImagePullPolicy: pullPolicy,
							Env: []corev1.EnvVar{
								{
									Name:  "OVSX_REGISTRY_URL",
									Value: ctx.CheCluster.Status.OpenVSXURL,
								},
								envFromSecret("OVSX_PAT", secretName, "userPAT"),
								{
									Name:  "NODE_EXTRA_CA_CERTS",
									Value: "/public-certs/tls-ca-bundle.pem",
								},
								{
									Name:  "EXTENSIONS_HASH",
									Value: extensionsHash,
								},
							},
							Command: []string{"/home/openvsx/publish-extensions.sh", "/home/openvsx/extensions/extensions.txt"},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "extensions",
									MountPath: "/home/openvsx/extensions",
									ReadOnly:  true,
								},
								{
									Name:      "ca-certs",
									MountPath: "/public-certs",
									ReadOnly:  true,
								},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "extensions",
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: extensionsConfigMapName,
									},
								},
							},
						},
						{
							Name: "ca-certs",
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: tls.CheMergedCABundleCertsCMName,
									},
								},
							},
						},
					},
				},
			},
			Parallelism:  &parallelism,
			BackoffLimit: &backoffLimit,
			Completions:  &completions,
		},
	}

	return deploy.Sync(ctx, job, deploy.JobDiffOpts)
}

func envFromSecret(envName, secretName, key string) corev1.EnvVar {
	return corev1.EnvVar{
		Name: envName,
		ValueFrom: &corev1.EnvVarSource{
			SecretKeyRef: &corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{Name: secretName},
				Key:                  key,
			},
		},
	}
}
