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
	orgv1 "github.com/eclipse-che/che-operator/pkg/apis/org/v1"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

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
