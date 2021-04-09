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
	"net/http"
	"strconv"

	orgv1 "github.com/eclipse-che/che-operator/pkg/apis/org/v1"
	"github.com/eclipse-che/che-operator/pkg/backup_servers"
	"github.com/eclipse-che/che-operator/pkg/deploy"
	"github.com/eclipse-che/che-operator/pkg/util"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	InternalBackupServerType      = "internal"
	InternalBackupServerComponent = "che-backup-rest-server"

	BackupServerRepoPasswordSecretName = "backup-rest-server-repo-password"
	backupServerDeploymentName         = "backup-rest-server-deployment"
	backupServerPodName                = "backup-rest-server-pod"
	backupServerServiceName            = "backup-rest-server-service"
	backupServerPort                   = 8000
)

// ConfigureInternalBackupServer check for existance of internal REST backup server and deploys it if missing.
// If something is broken in the internal backup server configuration,
// then it will be recreated, but all data will be lost.
func ConfigureInternalBackupServer(bctx *BackupContext) (bool, error) {
	taskList := []func(*BackupContext) (bool, error){
		ensureInternalBackupServerDeploymentExist,
		ensureInternalBackupServerServiceExists,
		ensureInternalBackupServerSecretExists,
		ensureInternalBackupServerConfiguredAndCurrent,
		ensureInternalBackupServerRepositoryInitialized,
	}

	for _, task := range taskList {
		done, err := task(bctx)
		if !(done && err == nil) {
			return done, err
		}
	}

	return true, nil
}

func ensureInternalBackupServerDeploymentExist(bctx *BackupContext) (bool, error) {
	backupServerDeployment := &appsv1.Deployment{}
	namespacedName := types.NamespacedName{
		Namespace: bctx.namespace,
		Name:      backupServerDeploymentName,
	}
	err := bctx.r.client.Get(context.TODO(), namespacedName, backupServerDeployment)
	if err == nil {
		return true, nil
	}
	if !errors.IsNotFound(err) {
		return false, err
	}

	// Get default configuration of the backup server deployment
	backupServerDeployment, err = getBackupServerDeploymentSpec(bctx)
	if err != nil {
		return false, err
	}
	// Create backup server deployment
	err = bctx.r.client.Create(context.TODO(), backupServerDeployment)
	if err != nil {
		return false, err
	}
	// Backup server created successfully, reconcile
	return false, nil
}

func getBackupServerDeploymentSpec(bctx *BackupContext) (*appsv1.Deployment, error) {
	labels, labelSelector := deploy.GetLabelsAndSelector(bctx.cheCR, InternalBackupServerComponent)
	replicas := int32(1)

	deployment := &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Deployment",
			APIVersion: "apps/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      backupServerDeploymentName,
			Namespace: bctx.namespace,
			Labels:    labels,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{MatchLabels: labelSelector},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
					Name:   backupServerPodName,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:            backupServerPodName,
							Image:           "mm4eche/rest-server:latest", // TODO replace with official image
							ImagePullPolicy: "Always",
							Ports: []corev1.ContainerPort{
								{
									Name:          "rest",
									ContainerPort: backupServerPort,
									Protocol:      "TCP",
								},
							},
							ReadinessProbe: &corev1.Probe{
								Handler: corev1.Handler{
									TCPSocket: &corev1.TCPSocketAction{
										Port: intstr.IntOrString{
											Type:   intstr.Int,
											IntVal: int32(backupServerPort),
										},
									},
								},
								InitialDelaySeconds: 1,
								FailureThreshold:    10,
								TimeoutSeconds:      1,
								SuccessThreshold:    1,
								PeriodSeconds:       1,
							},
							SecurityContext: &corev1.SecurityContext{
								Capabilities: &corev1.Capabilities{
									Drop: []corev1.Capability{"ALL"},
								},
							},
						},
					},
					RestartPolicy: "Always",
				},
			},
		},
	}

	// Set CheClusterBackup instance as the owner and controller
	if err := controllerutil.SetControllerReference(bctx.backupCR, deployment, bctx.r.scheme); err != nil {
		return nil, err
	}

	return deployment, nil
}

func ensureInternalBackupServerServiceExists(bctx *BackupContext) (bool, error) {
	backupServerService := &corev1.Service{}
	namespacedName := types.NamespacedName{
		Namespace: bctx.namespace,
		Name:      backupServerServiceName,
	}
	err := bctx.r.client.Get(context.TODO(), namespacedName, backupServerService)
	if err == nil {
		// Backup server service already exists, do nothing
		return true, nil
	}
	if !errors.IsNotFound(err) {
		return false, err
	}

	// Backup server service doesn't exists, create it
	backupServerService, err = getBackupServerServiceSpec(bctx)
	if err != nil {
		return false, err
	}
	// Create backup server service
	err = bctx.r.client.Create(context.TODO(), backupServerService)
	if err != nil {
		return false, err
	}
	// Backup server service created successfully, reconcile
	return false, nil
}

func getBackupServerServiceSpec(bctx *BackupContext) (*corev1.Service, error) {
	labels := deploy.GetLabels(bctx.cheCR, InternalBackupServerComponent)

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
			Namespace: bctx.namespace,
			Labels:    labels,
		},
		Spec: corev1.ServiceSpec{
			Ports:    ports,
			Selector: labels,
		},
	}

	// Set CheClusterBackup instance as the owner and controller
	if err := controllerutil.SetControllerReference(bctx.backupCR, service, bctx.r.scheme); err != nil {
		return nil, err
	}

	return service, nil
}

func ensureInternalBackupServerSecretExists(bctx *BackupContext) (bool, error) {
	// Check if the secret with restic repository password exists
	repoPasswordSecret := &corev1.Secret{}
	namespacedName := types.NamespacedName{
		Namespace: bctx.namespace,
		Name:      bctx.backupCR.Spec.Servers.Internal.PasswordSecretRef,
	}
	err := bctx.r.client.Get(context.TODO(), namespacedName, repoPasswordSecret)
	if err == nil {
		return true, nil
	}
	if !errors.IsNotFound(err) {
		return false, err
	}

	repoPassword := util.GeneratePasswd(12)
	repoPasswordSecret, err = getRepoPasswordSecretSpec(bctx, repoPassword)
	if err != nil {
		return false, err
	}
	err = bctx.r.client.Create(context.TODO(), repoPasswordSecret)
	if err != nil {
		return false, err
	}
	// Reconcile after secret creation
	return false, nil
}

func getRepoPasswordSecretSpec(bctx *BackupContext, password string) (*corev1.Secret, error) {
	labels := deploy.GetLabels(bctx.cheCR, InternalBackupServerComponent)
	data := map[string][]byte{"repo-password": []byte(password)}

	secret := &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Secret",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      BackupServerRepoPasswordSecretName,
			Namespace: bctx.namespace,
			Labels:    labels,
		},
		Data: data,
	}

	if err := controllerutil.SetControllerReference(bctx.backupCR, secret, bctx.r.scheme); err != nil {
		return nil, err
	}

	return secret, nil
}

// ensureInternalBackupServerConfiguredAndCurrent makes sure that current backup server is configured to internal rest server.
func ensureInternalBackupServerConfiguredAndCurrent(bctx *BackupContext) (bool, error) {
	backupCR := bctx.backupCR

	shouldUpdateCR := false

	if backupCR.Spec.ServerType != InternalBackupServerType {
		backupCR.Spec.ServerType = InternalBackupServerType
		shouldUpdateCR = true
	}

	expectedInternalRestServerConfig := orgv1.RestServerConfig{
		Protocol: "http",
		// Hostname: backupServerServiceName,
		Hostname: "rest.192.168.99.254.nip.io", // TODO revert debug code
		Port:     strconv.Itoa(backupServerPort),
		Repo:     "che",
		RepoPassword: orgv1.RepoPassword{
			PasswordSecretRef: BackupServerRepoPasswordSecretName,
		},
	}
	if backupCR.Spec.Servers.Internal != expectedInternalRestServerConfig {
		backupCR.Spec.Servers.Internal = expectedInternalRestServerConfig
		shouldUpdateCR = true
	}

	if shouldUpdateCR {
		err := bctx.r.UpdateCR(backupCR)
		if err != nil {
			return false, err
		}
		// Reconcile after CR update
		return false, nil
	}

	return true, nil
}

func ensureInternalBackupServerRepositoryInitialized(bctx *BackupContext) (bool, error) {
	restServer := backup_servers.RestServer{Config: bctx.backupCR.Spec.Servers.Internal}
	done, err := restServer.PrepareConfiguration(bctx.r.client, bctx.namespace)
	if err != nil || !done {
		return done, err
	}

	response, err := http.Head(restServer.ResticClient.RepoUrl + "/config")
	if err != nil || response.ContentLength == 0 {
		// Cannot read the repository, probably it doesn't exist
		return restServer.InitRepository()
	}

	return true, nil
}
