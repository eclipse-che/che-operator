//
// Copyright (c) 2019-2022 Red Hat, Inc.
// This program and the accompanying materials are made
// available under the terms of the Eclipse Public License 2.0
// which is available at https://www.eclipse.org/legal/epl-2.0/
//
// SPDX-License-Identifier: EPL-2.0
//
// Contributors:
//   Red Hat, Inc. - initial API and implementation
//

package v2

import (
	"context"
	"fmt"

	k8shelper "github.com/eclipse-che/che-operator/pkg/common/k8s-helper"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

// log is for logging in this package.
var (
	logger = ctrl.Log.WithName("webhook")
)

func (r *CheCluster) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

var _ webhook.Validator = &CheCluster{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *CheCluster) ValidateCreate() error {
	return ensureSingletonCheCluster()
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *CheCluster) ValidateUpdate(old runtime.Object) error {
	return nil
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *CheCluster) ValidateDelete() error {
	return nil
}

func ensureSingletonCheCluster() error {
	client := k8shelper.New().GetClient()
	utilruntime.Must(AddToScheme(client.Scheme()))

	che := &CheClusterList{}
	err := client.List(context.TODO(), che)
	if err != nil {
		logger.Error(err, "Failed to list CheCluster Custom Resources.")
	}

	if len(che.Items) != 0 {
		return fmt.Errorf("only one CheCluster is allowed")
	}

	return nil
}
