//
// Copyright (c) 2019-2023 Red Hat, Inc.
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

	"github.com/eclipse-che/che-operator/pkg/common/chetypes"
	defaults "github.com/eclipse-che/che-operator/pkg/common/operator-defaults"
	"github.com/eclipse-che/che-operator/pkg/common/utils"
	"github.com/eclipse-che/che-operator/pkg/deploy"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
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

var consoleLinkDiffOpts = cmp.Options{
	cmpopts.IgnoreFields(consolev1.ConsoleLink{}, "TypeMeta", "ObjectMeta"),
}

type ConsoleLinkReconciler struct {
	deploy.Reconcilable
}

func NewConsoleLinkReconciler() *ConsoleLinkReconciler {
	return &ConsoleLinkReconciler{}
}

func (c *ConsoleLinkReconciler) Reconcile(ctx *chetypes.DeployContext) (reconcile.Result, bool, error) {
	if !utils.IsK8SResourceServed(ctx.ClusterAPI.DiscoveryClient, ConsoleLinksResourceName) {
		logrus.Debug("Console link won't be created. ConsoleLinks is not supported by kubernetes cluster.")
		return reconcile.Result{}, true, nil
	}

	done, err := c.syncConsoleLink(ctx)
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

func (c *ConsoleLinkReconciler) syncConsoleLink(ctx *chetypes.DeployContext) (bool, error) {
	if err := deploy.AppendFinalizer(ctx, ConsoleLinkFinalizerName); err != nil {
		return false, err
	}

	consoleLinkSpec := c.getConsoleLinkSpec(ctx)
	return deploy.Sync(ctx, consoleLinkSpec, consoleLinkDiffOpts)
}

func (c *ConsoleLinkReconciler) getConsoleLinkSpec(ctx *chetypes.DeployContext) *consolev1.ConsoleLink {
	consoleLink := &consolev1.ConsoleLink{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ConsoleLink",
			APIVersion: consolev1.GroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: defaults.GetConsoleLinkName(),
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
