//
// Copyright (c) 2012-2019 Red Hat, Inc.
// This program and the accompanying materials are made
// available under the terms of the Eclipse Public License 2.0
// which is available at https://www.eclipse.org/legal/epl-2.0/
//
// SPDX-License-Identifier: EPL-2.0
//
// Contributors:
//   Red Hat, Inc. - initial API and implementation
//
package postgres

import (
	"fmt"

	orgv1 "github.com/eclipse-che/che-operator/api/v1"
	"github.com/eclipse-che/che-operator/pkg/deploy"
	identity_provider "github.com/eclipse-che/che-operator/pkg/deploy/identity-provider"
	"github.com/eclipse-che/che-operator/pkg/util"
	"github.com/sirupsen/logrus"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
)

type Postgres struct {
	deployContext *deploy.DeployContext
	isMultiUser   bool
}

func NewPostgres(deployContext *deploy.DeployContext) *Postgres {
	return &Postgres{
		deployContext: deployContext,
		isMultiUser:   deploy.GetCheMultiUser(deployContext.CheCluster) == "true",
	}
}

func (p *Postgres) SyncAll() (bool, error) {
	done, err := p.SyncService()
	if !done {
		return false, err
	}

	done, err = p.SyncPVC()
	if !done {
		return false, err
	}

	done, err = p.SyncDeployment()
	if !done {
		return false, err
	}

	if !p.deployContext.CheCluster.Status.DbProvisoned {
		if !util.IsTestMode() { // ignore in tests
			done, err = p.ProvisionDB()
			if !done {
				return false, err
			}
		}
	}

	return true, nil
}

func (p *Postgres) SyncService() (bool, error) {
	if !p.isMultiUser {
		return deploy.DeleteNamespacedObject(p.deployContext, deploy.PostgresName, &corev1.Service{})
	}
	return deploy.SyncServiceToCluster(p.deployContext, deploy.PostgresName, []string{deploy.PostgresName}, []int32{5432}, deploy.PostgresName)
}

func (p *Postgres) SyncPVC() (bool, error) {
	if !p.isMultiUser {
		return deploy.DeleteNamespacedObject(p.deployContext, deploy.DefaultPostgresVolumeClaimName, &corev1.PersistentVolumeClaim{})
	}

	done, err := deploy.SyncPVCToCluster(p.deployContext, deploy.DefaultPostgresVolumeClaimName, "1Gi", deploy.PostgresName)
	if !done {
		if err == nil {
			logrus.Infof("Waiting on pvc '%s' to be bound. Sometimes PVC can be bound only when the first consumer is created.", deploy.DefaultPostgresVolumeClaimName)
		}
	}
	return done, err
}

func (p *Postgres) SyncDeployment() (bool, error) {
	if !p.isMultiUser {
		return deploy.DeleteNamespacedObject(p.deployContext, deploy.PostgresName, &appsv1.Deployment{})
	}

	clusterDeployment := &appsv1.Deployment{}
	exists, err := deploy.GetNamespacedObject(p.deployContext, deploy.PostgresName, clusterDeployment)
	if err != nil {
		return false, err
	}

	if !exists {
		clusterDeployment = nil
	}

	specDeployment, err := p.GetDeploymentSpec(clusterDeployment)
	if err != nil {
		return false, err
	}

	return deploy.SyncDeploymentSpecToCluster(p.deployContext, specDeployment, deploy.DefaultDeploymentDiffOpts)
}

func (p *Postgres) ProvisionDB() (bool, error) {
	identityProviderPostgresPassword := p.deployContext.CheCluster.Spec.Auth.IdentityProviderPostgresPassword
	identityProviderPostgresSecret := p.deployContext.CheCluster.Spec.Auth.IdentityProviderPostgresSecret
	if identityProviderPostgresSecret != "" {
		secret := &corev1.Secret{}
		exists, err := deploy.GetNamespacedObject(p.deployContext, identityProviderPostgresSecret, secret)
		if err != nil {
			return false, err
		} else if !exists {
			return false, fmt.Errorf("Secret '%s' not found", identityProviderPostgresSecret)
		}
		identityProviderPostgresPassword = string(secret.Data["password"])
	}

	_, err := util.K8sclient.ExecIntoPod(
		p.deployContext.CheCluster,
		deploy.PostgresName,
		func(cr *orgv1.CheCluster) (string, error) {
			return identity_provider.GetPostgresProvisionCommand(identityProviderPostgresPassword), nil
		},
		"create Keycloak DB, user, privileges")
	if err != nil {
		return false, err
	}

	p.deployContext.CheCluster.Status.DbProvisoned = true
	err = deploy.UpdateCheCRStatus(p.deployContext, "status: provisioned with DB and user", "true")
	if err != nil {
		return false, err
	}

	return true, nil
}
