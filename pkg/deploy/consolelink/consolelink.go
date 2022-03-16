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

	"github.com/eclipse-che/che-operator/pkg/deploy"
	"github.com/eclipse-che/che-operator/pkg/util"
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

func (c *ConsoleLinkReconciler) Reconcile(ctx *deploy.DeployContext) (reconcile.Result, bool, error) {
	if !util.IsOpenShift4 || !util.HasK8SResourceObject(ctx.ClusterAPI.DiscoveryClient, ConsoleLinksResourceName) {
		// console link is supported only on OpenShift >= 4.2
		logrus.Debug("Console link won't be created. Consolelinks is not supported by OpenShift cluster.")
		return reconcile.Result{}, true, nil
	}

	done, err := c.createConsoleLink(ctx)
	if !done {
		return reconcile.Result{Requeue: true}, false, err
	}

	return reconcile.Result{}, true, nil
}

func (c *ConsoleLinkReconciler) Finalize(ctx *deploy.DeployContext) bool {
	if err := deploy.DeleteObjectWithFinalizer(ctx, client.ObjectKey{Name: deploy.DefaultConsoleLinkName()}, &consolev1.ConsoleLink{}, ConsoleLinkFinalizerName); err != nil {
		logrus.Errorf("Error deleting finalizer: %v", err)
		return false
	}
	return true
}

func (c *ConsoleLinkReconciler) createConsoleLink(ctx *deploy.DeployContext) (bool, error) {
	consoleLinkSpec := c.getConsoleLinkSpec(ctx)
	_, err := deploy.CreateIfNotExists(ctx, consoleLinkSpec)
	if err != nil {
		return false, err
	}

	consoleLink := &consolev1.ConsoleLink{}
	exists, err := deploy.Get(ctx, client.ObjectKey{Name: deploy.DefaultConsoleLinkName()}, consoleLink)
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

func (c *ConsoleLinkReconciler) getConsoleLinkSpec(ctx *deploy.DeployContext) *consolev1.ConsoleLink {
	consoleLink := &consolev1.ConsoleLink{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ConsoleLink",
			APIVersion: consolev1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: deploy.DefaultConsoleLinkName(),
			Annotations: map[string]string{
				deploy.CheEclipseOrgNamespace: ctx.CheCluster.Namespace,
			},
		},
		Spec: consolev1.ConsoleLinkSpec{
			Link: consolev1.Link{
				Href: "https://" + ctx.CheHost,
				Text: deploy.DefaultConsoleLinkDisplayName()},
			Location: consolev1.ApplicationMenu,
			ApplicationMenu: &consolev1.ApplicationMenuSpec{
				Section:  deploy.DefaultConsoleLinkSection(),
				ImageURL: fmt.Sprintf("https://%s%s", ctx.CheHost, deploy.DefaultConsoleLinkImage()),
			},
		},
	}

	return consoleLink
}
