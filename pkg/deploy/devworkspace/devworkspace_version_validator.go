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

package devworkspace

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/blang/semver/v4"
	"github.com/eclipse-che/che-operator/pkg/common/chetypes"
	"github.com/eclipse-che/che-operator/pkg/common/constants"
	"github.com/eclipse-che/che-operator/pkg/common/reconciler"
	operatorsv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type DevWorkspaceVersionValidator struct {
	reconciler.Reconcilable
	minimumDwVersion string
}

func NewDevWorkspaceVersionValidator(minimumDwVersion string) *DevWorkspaceVersionValidator {
	return &DevWorkspaceVersionValidator{
		minimumDwVersion: minimumDwVersion,
	}
}

func (v *DevWorkspaceVersionValidator) Reconcile(ctx *chetypes.DeployContext) (reconcile.Result, bool, error) {
	if err := v.ensureDevWorkspaceVersion(ctx); err != nil {
		return reconcile.Result{}, false, err
	}

	return reconcile.Result{}, true, nil
}

func (v *DevWorkspaceVersionValidator) Finalize(ctx *chetypes.DeployContext) bool {
	return true
}

func (v *DevWorkspaceVersionValidator) ensureDevWorkspaceVersion(ctx *chetypes.DeployContext) error {
	subscriptions := &operatorsv1alpha1.SubscriptionList{}
	if err := ctx.ClusterAPI.NonCachingClient.List(context.TODO(), subscriptions); err != nil {
		return err
	}

	idx := slices.IndexFunc(
		subscriptions.Items,
		func(subscription operatorsv1alpha1.Subscription) bool {
			return subscription.Spec.Package == constants.DevWorkspaceOperatorName
		},
	)

	if idx == -1 {
		return fmt.Errorf(constants.DevWorkspaceOperatorNotExistsErrorMsg)
	}

	subscription := subscriptions.Items[idx]

	installedCSV := subscription.Status.InstalledCSV
	if installedCSV == "" {
		return fmt.Errorf("DevWorkspace Operator CSV is not installed yet")
	}

	// Extract version from CSV name (e.g., "devworkspace-operator.v0.40.0-dev.3" -> "0.40.0-dev.3")
	baseVersion, _, err := extractVersionFromCSV(installedCSV)
	if err != nil {
		return fmt.Errorf("failed to extract version from CSV %s: %w", installedCSV, err)
	}

	// Parse installed and required versions
	installedSemver, err := semver.Parse(baseVersion)
	if err != nil {
		return fmt.Errorf("failed to parse installed DevWorkspace Operator version %s: %w", baseVersion, err)
	}

	requiredSemver, err := semver.Parse(v.minimumDwVersion)
	if err != nil {
		return fmt.Errorf("failed to parse required DevWorkspace Operator version %s: %w", v.minimumDwVersion, err)
	}

	// Compare versions
	if installedSemver.LT(requiredSemver) {
		return fmt.Errorf("DevWorkspace Operator version %s is installed, but Eclipse Che requires version %s or higher. Please upgrade the DevWorkspace Operator", baseVersion, v.minimumDwVersion)
	}

	return nil
}

func extractVersionFromCSV(csvName string) (string, string, error) {
	idx := strings.LastIndex(csvName, ".v")
	if idx == -1 {
		return "", "", fmt.Errorf("CSV name does not contain version prefix '.v': %s", csvName)
	}

	version := csvName[idx+2:] // +2 to skip ".v"
	if version == "" {
		return "", "", fmt.Errorf("CSV name does not contain version after '.v': %s", csvName)
	}

	dashIdx := strings.LastIndex(version, "-")
	if dashIdx != -1 {
		baseVersion := version[:dashIdx]
		devVersion := version[dashIdx+1:]
		return baseVersion, devVersion, nil
	}

	return version, "", nil
}
