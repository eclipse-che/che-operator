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
	"time"

	"github.com/eclipse-che/che-operator/pkg/common/chetypes"
	k8sclient "github.com/eclipse-che/che-operator/pkg/common/k8s-client"
	defaults "github.com/eclipse-che/che-operator/pkg/common/operator-defaults"
	"github.com/eclipse-che/che-operator/pkg/common/reconciler"
	"github.com/eclipse-che/che-operator/pkg/deploy"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	consolev1 "github.com/openshift/api/console/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	ConsoleLinkFinalizerName = "consolelink.finalizers.che.eclipse.org"
)

var (
	logger              = ctrl.Log.WithName("consolelink")
	consoleLinkDiffOpts = cmp.Options{
		cmpopts.IgnoreFields(consolev1.ConsoleLink{}, "TypeMeta", "ObjectMeta"),
	}
)

type ConsoleLinkReconciler struct {
	reconciler.Reconcilable
}

func NewConsoleLinkReconciler() *ConsoleLinkReconciler {
	return &ConsoleLinkReconciler{}
}

func (c *ConsoleLinkReconciler) Reconcile(cheCtx *chetypes.CheContext) (reconcile.Result, bool, error) {
	if err := c.syncConsoleLink(cheCtx); err != nil {
		return reconcile.Result{RequeueAfter: time.Second}, false, err
	}

	return reconcile.Result{}, true, nil
}

func (c *ConsoleLinkReconciler) Finalize(cheCtx *chetypes.CheContext) bool {
	if err := cheCtx.ClusterAPI.NonCachingClientWrapper.DeleteByKeyIgnoreNotFound(
		cheCtx.Context,
		client.ObjectKey{Name: defaults.GetConsoleLinkName()},
		&consolev1.ConsoleLink{},
	); err != nil {
		// failed to delete ConsoleLink, but it shouldn't prevent us from removing the finalizer
		logger.Error(err, "Failed to delete ConsoleLink", "name", defaults.GetConsoleLinkName())
	}

	if err := deploy.DeleteFinalizer(cheCtx, ConsoleLinkFinalizerName); err != nil {
		logger.Error(err, "Failed to delete finalizer", "finalizer", ConsoleLinkFinalizerName)
		return false
	}

	return true
}

func (c *ConsoleLinkReconciler) syncConsoleLink(cheCtx *chetypes.CheContext) error {
	if err := deploy.AppendFinalizer(cheCtx, ConsoleLinkFinalizerName); err != nil {
		return fmt.Errorf("failed to append finalizer %s: %w", ConsoleLinkFinalizerName, err)
	}

	consoleLinkSpec := c.getConsoleLinkSpec(cheCtx)

	// ConsoleLink is a cluster scoped object, so it can't have an owner reference
	// and must be synced with the non-caching client
	if err := cheCtx.ClusterAPI.NonCachingClientWrapper.Sync(
		cheCtx.Context,
		consoleLinkSpec,
		&k8sclient.SyncOptions{DiffOpts: consoleLinkDiffOpts},
	); err != nil {
		return fmt.Errorf("failed to sync ConsoleLink %s: %w", consoleLinkSpec.Name, err)
	}

	return nil
}

func (c *ConsoleLinkReconciler) getConsoleLinkSpec(cheCtx *chetypes.CheContext) *consolev1.ConsoleLink {
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
				Href: "https://" + cheCtx.CheHost,
				Text: defaults.GetConsoleLinkDisplayName()},
			Location: consolev1.ApplicationMenu,
			ApplicationMenu: &consolev1.ApplicationMenuSpec{
				Section:  defaults.GetConsoleLinkSection(),
				ImageURL: fmt.Sprintf("https://%s%s", cheCtx.CheHost, defaults.GetConsoleLinkImage()),
			},
		},
	}

	return consoleLink
}
