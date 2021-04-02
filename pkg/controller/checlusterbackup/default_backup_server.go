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
package checlusterbackup

import (
	"context"
	"strconv"

	orgv1 "github.com/eclipse-che/che-operator/pkg/apis/org/v1"
	"github.com/eclipse-che/che-operator/pkg/deploy"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	InternalBackupServerType = "internal"

	backupServerDeploymentName         = "backup-rest-server-deployment"
	backupServerPodName                = "backup-rest-server-pod"
	backupServerServiceName            = "backup-rest-server-service"
	backupServerRepoPasswordSecretName = "backup-rest-server-repo-password"
	backupServerPort                   = 8000
)

func (r *ReconcileCheClusterBackup) EnsureDefaultBackupServerDeploymentExists(backupCR *orgv1.CheClusterBackup) error {
	backupServerDeployment := &appsv1.Deployment{}
	namespacedName := types.NamespacedName{
		Namespace: backupCR.GetNamespace(),
		Name:      backupServerDeploymentName,
	}
	err := r.client.Get(context.TODO(), namespacedName, backupServerDeployment)
	if err == nil {
		// Backup server already exists, do nothing
		return nil
	}
	if !errors.IsNotFound(err) {
		return err
	}

	// Backup server doesn't exists, create it
	backupServerDeployment, err = r.getBackupServerDeploymentSpec(backupCR.GetNamespace())
	if err != nil {
		return err
	}
	// Set CheClusterBackup instance as the owner and controller
	if err := controllerutil.SetControllerReference(backupCR, backupServerDeployment, r.scheme); err != nil {
		return err
	}
	// Create backup server deployment
	err = r.client.Create(context.TODO(), backupServerDeployment)
	if err != nil {
		return err
	}
	// Backup server created successfully
	return nil
}

func (r *ReconcileCheClusterBackup) getBackupServerDeploymentSpec(namespace string) (*appsv1.Deployment, error) {
	cheCR, err := r.GetCheCR(namespace)
	if err != nil {
		return nil, err
	}

	labels, labelSelector := deploy.GetLabelsAndSelector(cheCR, deploy.PostgresName)
	replicas := int32(1)
	terminationGracePeriodSeconds := int64(30)

	deployment := &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Deployment",
			APIVersion: "apps/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      backupServerDeploymentName,
			Namespace: namespace,
			Labels:    labels,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{MatchLabels: labelSelector},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:            backupServerPodName,
							Image:           "restic/rest-server:latest",
							ImagePullPolicy: "IfNotPresent",
							Command:         []string{"rest-server", "--no-auth"},
							Ports: []corev1.ContainerPort{
								{
									Name:          "rest",
									ContainerPort: backupServerPort,
									Protocol:      "TCP",
								},
							},
							ReadinessProbe: &corev1.Probe{
								Handler: corev1.Handler{
									HTTPGet: &corev1.HTTPGetAction{
										Path: "/",
										Port: intstr.IntOrString{
											Type:   intstr.Int,
											IntVal: int32(backupServerPort),
										},
										Scheme: corev1.URISchemeHTTP,
									},
								},
								InitialDelaySeconds: 3,
								FailureThreshold:    10,
								TimeoutSeconds:      3,
								SuccessThreshold:    1,
								PeriodSeconds:       10,
							},
							SecurityContext: &corev1.SecurityContext{
								Capabilities: &corev1.Capabilities{
									Drop: []corev1.Capability{"ALL"},
								},
							},
						},
					},
					RestartPolicy:                 "Always",
					TerminationGracePeriodSeconds: &terminationGracePeriodSeconds,
				},
			},
		},
	}

	return deployment, nil
}

func (r *ReconcileCheClusterBackup) EnsureDefaultBackupServerServiceExists(backupCR *orgv1.CheClusterBackup) error {
	backupServerService := &corev1.Service{}
	namespacedName := types.NamespacedName{
		Namespace: backupCR.GetNamespace(),
		Name:      backupServerServiceName,
	}
	err := r.client.Get(context.TODO(), namespacedName, backupServerService)
	if err == nil {
		// Backup server service already exists, do nothing
		return nil
	}
	if !errors.IsNotFound(err) {
		return err
	}

	// Backup server service doesn't exists, create it
	backupServerService, err = r.getBackupServerServiceSpec(backupCR.GetNamespace())
	if err != nil {
		return err
	}
	// Set CheClusterBackup instance as the owner and controller
	if err := controllerutil.SetControllerReference(backupCR, backupServerService, r.scheme); err != nil {
		return err
	}
	// Create backup server service
	err = r.client.Create(context.TODO(), backupServerService)
	if err != nil {
		return err
	}
	// Backup server service created successfully
	return nil
}

func (r *ReconcileCheClusterBackup) getBackupServerServiceSpec(namespace string) (*corev1.Service, error) {
	cheCR, err := r.GetCheCR(namespace)
	if err != nil {
		return nil, err
	}

	labels, _ := deploy.GetLabelsAndSelector(cheCR, deploy.PostgresName)

	port := corev1.ServicePort{
		Name:     backupServerServiceName + "-port",
		Port:     backupServerPort,
		Protocol: "TCP",
	}
	ports := []corev1.ServicePort{}
	ports = append(ports, port)

	service := &corev1.Service{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Service",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      backupServerServiceName,
			Namespace: namespace,
			Labels:    labels,
		},
		Spec: corev1.ServiceSpec{
			Ports:    ports,
			Selector: labels,
		},
	}
	return service, nil
}

// EnsureInternalBackupServerConfigured makes sure that current backup server is configured to internal rest server.
func (r *ReconcileCheClusterBackup) EnsureInternalBackupServerConfigured(backupCR *orgv1.CheClusterBackup) error {
	shouldUpdate := false

	if backupCR.Spec.ServerType != InternalBackupServerType {
		backupCR.Spec.ServerType = InternalBackupServerType
		shouldUpdate = true
	}

	expectedInternalRestServerConfig := orgv1.RestServerConfing{
		Protocol: "http",
		Hostname: backupServerServiceName,
		Port:     strconv.Itoa(backupServerPort),
		Username: "user",
		RepoPassword: orgv1.RepoPassword{
			RepoPasswordSecretRef: backupServerRepoPasswordSecretName,
		},
	}
	if backupCR.Spec.Servers.Internal != expectedInternalRestServerConfig {
		backupCR.Spec.Servers.Internal = expectedInternalRestServerConfig
		shouldUpdate = true
	}

	if shouldUpdate {
		err := r.UpdateCR(backupCR)
		if err != nil {
			return err
		}
	}

	return nil
}
