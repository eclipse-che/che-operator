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

package consolelink

import (
	"fmt"

	"github.com/eclipse-che/che-operator/pkg/common/chelabels"
	"github.com/eclipse-che/che-operator/pkg/common/chetypes"
	defaults "github.com/eclipse-che/che-operator/pkg/common/operator-defaults"
	"github.com/eclipse-che/che-operator/pkg/provision/resources"
	"github.com/eclipse-che/che-operator/pkg/reconciler"
	consolev1 "github.com/openshift/api/console/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type ConsoleLinkReconciler struct {
	reconciler.Reconcilable
}

func NewReconciler() *ConsoleLinkReconciler {
	return &ConsoleLinkReconciler{}
}

func (c *ConsoleLinkReconciler) Reconcile(cheCtx *chetypes.CheContext) (reconcile.Result, bool, error) {
	if err := c.syncAll(cheCtx); err != nil {
		return reconcile.Result{},
			false,
			fmt.Errorf("failed to reconcile ConsoleLink: %w", err)
	}

	return reconcile.Result{}, true, nil
}

func (c *ConsoleLinkReconciler) Finalize(cheCtx *chetypes.CheContext) error {
	return c.deleteAll(cheCtx)
}

func (c *ConsoleLinkReconciler) syncAll(cheCtx *chetypes.CheContext) error {
	consoleLink := buildConsoleLink(cheCtx)
	return resources.ConsoleLink.Sync(cheCtx, consoleLink)
}

func (c *ConsoleLinkReconciler) deleteAll(cheCtx *chetypes.CheContext) error {
	return resources.ConsoleLink.Delete(cheCtx, defaults.GetConsoleLinkName())
}

func buildConsoleLink(cheCtx *chetypes.CheContext) *consolev1.ConsoleLink {
	href := fmt.Sprintf("https://%s", cheCtx.CheHost)
	imageURL := fmt.Sprintf("https://%s%s", cheCtx.CheHost, defaults.GetConsoleLinkImage())

	return &consolev1.ConsoleLink{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ConsoleLink",
			APIVersion: consolev1.GroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:   defaults.GetConsoleLinkName(),
			Labels: chelabels.GetStandardLabels(defaults.GetCheFlavor()),
		},
		Spec: consolev1.ConsoleLinkSpec{
			Link: consolev1.Link{
				Href: href,
				Text: defaults.GetConsoleLinkDisplayName()},
			Location: consolev1.ApplicationMenu,
			ApplicationMenu: &consolev1.ApplicationMenuSpec{
				Section:  defaults.GetConsoleLinkSection(),
				ImageURL: imageURL,
			},
		},
	}
}
