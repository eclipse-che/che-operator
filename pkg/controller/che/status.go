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
package che

import (
	orgv1 "github.com/eclipse/che-operator/pkg/apis/org/v1"
	"github.com/eclipse/che-operator/pkg/deploy"
	"github.com/eclipse/che-operator/pkg/util"
	"github.com/sirupsen/logrus"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	AvailableStatus               = "Available"
	UnavailableStatus             = "Unavailable"
	RollingUpdateInProgressStatus = "Available: Rolling update in progress"
)

func (r *ReconcileChe) SetCheAvailableStatus(instance *orgv1.CheCluster, request reconcile.Request, protocol string, cheHost string) (err error) {
	cheFlavor := util.GetValue(instance.Spec.Server.CheFlavor, deploy.DefaultCheFlavor)
	name := "Eclipse Che"
	if cheFlavor == "codeready" {
		name = "CodeReady Workspaces"
	}
	keycloakURL := instance.Spec.Auth.IdentityProviderURL
	instance.Status.KeycloakURL = keycloakURL
	if err := r.UpdateCheCRStatus(instance, "Keycloak URL status", keycloakURL); err != nil {
		instance, _ = r.GetCR(request)
		return err
	}
	instance.Status.CheClusterRunning = AvailableStatus
	if err := r.UpdateCheCRStatus(instance, "status: "+name+" server", AvailableStatus); err != nil {
		instance, _ = r.GetCR(request)
		return err
	}
	instance.Status.CheURL = protocol + "://" + cheHost
	if err := r.UpdateCheCRStatus(instance, name+" server URL", protocol+"://"+cheHost); err != nil {
		instance, _ = r.GetCR(request)
		return err
	}
	logrus.Infof(name+" is now available at: %s://%s", protocol, cheHost)
	return nil

}

func (r *ReconcileChe) SetCheUnavailableStatus(instance *orgv1.CheCluster, request reconcile.Request) (err error) {
	if instance.Status.CheClusterRunning != UnavailableStatus {
		instance.Status.CheClusterRunning = UnavailableStatus
		if err := r.UpdateCheCRStatus(instance, "status: Che API", UnavailableStatus); err != nil {
			instance, _ = r.GetCR(request)
			return err
		}
	}
	return nil
}

func (r *ReconcileChe) SetStatusDetails(instance *orgv1.CheCluster, request reconcile.Request, reason string, message string, helpLink string) (err error) {
	if reason != instance.Status.Reason {
		instance.Status.Reason = reason
		if err := r.UpdateCheCRStatus(instance, "status: Reason", reason); err != nil {
			instance, _ = r.GetCR(request)
			return err
		}
	}
	if message != instance.Status.Message {
		instance.Status.Message = message
		if err := r.UpdateCheCRStatus(instance, "status: Message", message); err != nil {
			instance, _ = r.GetCR(request)
			return err
		}
	}
	if helpLink != instance.Status.HelpLink {
		instance.Status.HelpLink = helpLink
		if err := r.UpdateCheCRStatus(instance, "status: HelpLink", message); err != nil {
			instance, _ = r.GetCR(request)
			return err
		}
	}
	return nil
}

func (r *ReconcileChe) SetCheRollingUpdateStatus(instance *orgv1.CheCluster, request reconcile.Request) (err error) {

	instance.Status.CheClusterRunning = RollingUpdateInProgressStatus
	if err := r.UpdateCheCRStatus(instance, "status", RollingUpdateInProgressStatus); err != nil {
		instance, _ = r.GetCR(request)
		return err
	}
	return nil
}
