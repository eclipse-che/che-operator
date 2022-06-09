//
// Copyright (c) 2019-2021 Red Hat, Inc.
// This program and the accompanying materials are made
// available under the terms of the Eclipse Public License 2.0
// which is available at https://www.eclipse.org/legal/epl-2.0/
//
// SPDX-License-Identifier: EPL-2.0
//
// Contributors:
//   Red Hat, Inc. - initial API and implementation
//
package consolelink

import (
	"fmt"
	"strings"

	"github.com/devfile/devworkspace-operator/pkg/infrastructure"
	"github.com/eclipse-che/che-operator/pkg/common/chetypes"
	"github.com/eclipse-che/che-operator/pkg/common/constants"
	defaults "github.com/eclipse-che/che-operator/pkg/common/operator-defaults"
	"github.com/eclipse-che/che-operator/pkg/common/utils"
	"github.com/eclipse-che/che-operator/pkg/deploy"
	consolev1 "github.com/openshift/api/console/v1"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	ConsoleLinkFinalizerName = "consolelink.finalizers.che.eclipse.org"
	ConsoleLinksResourceName = "consolelinks"
)

type ConsoleLinkReconciler struct {
	deploy.Reconcilable
}

func NewConsoleLinkReconciler() *ConsoleLinkReconciler {
	return &ConsoleLinkReconciler{}
}

func (c *ConsoleLinkReconciler) Reconcile(ctx *chetypes.DeployContext) (reconcile.Result, bool, error) {
	if !infrastructure.IsOpenShift() || !utils.IsK8SResourceServed(ctx.ClusterAPI.DiscoveryClient, ConsoleLinksResourceName) {
		logrus.Debug("Console link won't be created. Consolelinks is not supported by kubernetes cluster.")
		return reconcile.Result{}, true, nil
	}

	done, err := c.createConsoleLink(ctx)
	if !done {
		return reconcile.Result{Requeue: true}, false, err
	}

	return reconcile.Result{}, true, nil
}

func (c *ConsoleLinkReconciler) Finalize(ctx *chetypes.DeployContext) bool {
	if err := deploy.DeleteObjectWithFinalizer(ctx, client.ObjectKey{Name: defaults.GetConsoleLinkName()}, &consolev1.ConsoleLink{}, ConsoleLinkFinalizerName); err != nil {
		logrus.Errorf("Error deleting finalizer: %v", err)
		return false
	}
	return true
}

func (c *ConsoleLinkReconciler) createConsoleLink(ctx *chetypes.DeployContext) (bool, error) {
	consoleLinkSpec := c.getConsoleLinkSpec(ctx)
	_, err := deploy.CreateIfNotExists(ctx, consoleLinkSpec)
	if err != nil {
		return false, err
	}

	consoleLink := &consolev1.ConsoleLink{}
	exists, err := deploy.Get(ctx, client.ObjectKey{Name: defaults.GetConsoleLinkName()}, consoleLink)
	if !exists || err != nil {
		return false, err
	}

	// consolelink is for this specific instance of Eclipse Che
	if strings.Index(consoleLink.Spec.Link.Href, ctx.CheHost) != -1 {
		err = deploy.AppendFinalizer(ctx, ConsoleLinkFinalizerName)
		return err == nil, err
	}

	return true, nil
}

func (c *ConsoleLinkReconciler) getConsoleLinkSpec(ctx *chetypes.DeployContext) *consolev1.ConsoleLink {
	consoleLink := &consolev1.ConsoleLink{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ConsoleLink",
			APIVersion: consolev1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: defaults.GetConsoleLinkName(),
			Annotations: map[string]string{
				constants.CheEclipseOrgNamespace: ctx.CheCluster.Namespace,
			},
		},
		Spec: consolev1.ConsoleLinkSpec{
			Link: consolev1.Link{
				Href: "https://" + ctx.CheHost,
				Text: defaults.GetConsoleLinkDisplayName()},
			Location: consolev1.ApplicationMenu,
			ApplicationMenu: &consolev1.ApplicationMenuSpec{
				Section:  defaults.GetConsoleLinkSection(),
				ImageURL: fmt.Sprintf("https://%s%s", ctx.CheHost, defaults.GetConsoleLinkImage()),
			},
		},
	}

	return consoleLink
}
