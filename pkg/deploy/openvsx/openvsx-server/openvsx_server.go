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

	_ "embed"
	"fmt"
	"strings"

	"github.com/eclipse-che/che-operator/pkg/common/chetypes"
	"github.com/eclipse-che/che-operator/pkg/common/constants"
	"github.com/eclipse-che/che-operator/pkg/common/diffs"
	defaults "github.com/eclipse-che/che-operator/pkg/common/operator-defaults"
	"github.com/eclipse-che/che-operator/pkg/common/reconciler"
	"github.com/eclipse-che/che-operator/pkg/common/utils"
	"github.com/eclipse-che/che-operator/pkg/deploy"
	"github.com/eclipse-che/che-operator/pkg/deploy/openvsx"
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

//go:embed application.yml
var applicationConfig string

func NewOpenVSXServerReconciler() *OpenVSXServerReconciler {
	return &OpenVSXServerReconciler{}
}

const (
	extensionsConfigMapName = "openvsx-extensions"
	extensionPublishJobName = "openvsx-extension-publish"
)

func (r *OpenVSXServerReconciler) Reconcile(ctx *chetypes.DeployContext) (reconcile.Result, bool, error) {
	if !ctx.CheCluster.IsInternalOpenVSXRegistryEnabled() {
		ns := ctx.CheCluster.Namespace
		cw := ctx.ClusterAPI.ClientWrapper
		_ = cw.DeleteByKeyIgnoreNotFound(context.TODO(), types.NamespacedName{Name: constants.OpenVSXServerComponentName, Namespace: ns}, &appsv1.Deployment{})
		_ = cw.DeleteByKeyIgnoreNotFound(context.TODO(), types.NamespacedName{Name: constants.OpenVSXServerComponentName, Namespace: ns}, &corev1.Service{})
		_ = cw.DeleteByKeyIgnoreNotFound(context.TODO(), types.NamespacedName{Name: configMapName, Namespace: ns}, &corev1.ConfigMap{})
		_ = cw.DeleteByKeyIgnoreNotFound(context.TODO(), types.NamespacedName{Name: userSetupJobName, Namespace: ns}, &batchv1.Job{})
		_ = cw.DeleteByKeyIgnoreNotFound(context.TODO(), types.NamespacedName{Name: extensionPublishJobName, Namespace: ns}, &batchv1.Job{})
		_ = cw.DeleteByKeyIgnoreNotFound(context.TODO(), types.NamespacedName{Name: extensionsConfigMapName, Namespace: ns}, &corev1.ConfigMap{})
		_ = cw.DeleteByKeyIgnoreNotFound(context.TODO(), types.NamespacedName{Name: serverPVCName, Namespace: ns}, &corev1.PersistentVolumeClaim{})
		// TODO ingress
		// TODO Openvsxurl
		return reconcile.Result{}, true, nil
	}

	err := r.syncService(ctx)
	if err != nil {
		return reconcile.Result{}, false, fmt.Errorf("failed to sync Service %w", err)
	}

	err = r.syncConfigMap(ctx)
	if err != nil {
		return reconcile.Result{}, false, fmt.Errorf("failed to sync ConfigMap %w", err)
	}

	err = r.syncPVC(ctx)
	if err != nil {
		return reconcile.Result{}, false, fmt.Errorf("failed to sync PVC: %w", err)
	}

	done, err := r.syncDeployment(ctx)
	if !done {
		if err != nil {
			err = fmt.Errorf("failed to sync Deployment %w", err)
		}
		return reconcile.Result{}, false, err
	}

	err = r.syncIngress(ctx)
	if err != nil {
		return reconcile.Result{}, false, fmt.Errorf("failed to sync Ingress: %w", err)
	}

	err = r.syncOpenVSXURLStatus(ctx)
	if err != nil {
		return reconcile.Result{}, false, fmt.Errorf("failed to sync OpenVSXURL status: %w", err)
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
			Labels:    deploy.GetLabels(constants.OpenVSXServerComponentName),
		},
		Data: map[string]string{
			"extensions.txt": "",
		},
	}

	return deploy.Sync(ctx, cm, diffs.ConfigMapEnsureLabels)
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
		_ = ctx.ClusterAPI.ClientWrapper.DeleteByKeyIgnoreNotFound(context.TODO(), types.NamespacedName{Name: extensionPublishJobName, Namespace: ctx.CheCluster.Namespace}, &batchv1.Job{})
		return true, nil
	}

	if ctx.CheCluster.Status.OpenVSXURL == "" {
		return false, nil
	}

	extensionsHash := utils.ComputeHash256([]byte(extensionsList))

	existing := &batchv1.Job{}
	err = ctx.ClusterAPI.Client.Get(context.TODO(), types.NamespacedName{
		Name:      extensionPublishJobName,
		Namespace: ctx.CheCluster.Namespace,
	}, existing)
	if err == nil {
		for _, env := range existing.Spec.Template.Spec.Containers[0].Env {
			if env.Name == "EXTENSIONS_HASH" {
				if env.Value == extensionsHash {
					return true, nil
				}
				break
			}
		}
		_ = ctx.ClusterAPI.ClientWrapper.DeleteByKeyIgnoreNotFound(context.TODO(), types.NamespacedName{Name: extensionPublishJobName, Namespace: ctx.CheCluster.Namespace}, &batchv1.Job{})
	} else if !errors.IsNotFound(err) {
		return false, err
	}

	image := defaults.GetOpenVSXImage(ctx.CheCluster)
	pullPolicy := corev1.PullPolicy(utils.GetPullPolicyFromDockerImage(image))
	labels := deploy.GetLabels(constants.OpenVSXServerComponentName)
	backoffLimit := int32(3)
	parallelism := int32(1)
	completions := int32(1)
	terminationGracePeriodSeconds := int64(30)
	runAsNonRoot := true

	secretName := openvsx.GetCredentialsSecretName(ctx)

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
									Value: ctx.CheCluster.Status.OpenVSXURL, // TODO
								},
								envFromSecret("OVSX_PAT", secretName, "publisher-token"),
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
