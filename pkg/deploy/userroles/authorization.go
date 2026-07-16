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

package userroles

import (
	"context"
	"slices"

	chev2 "github.com/eclipse-che/che-operator/api/v2"
	"github.com/eclipse-che/che-operator/pkg/common/chetypes"
	"github.com/eclipse-che/che-operator/pkg/common/infrastructure"
	userv1 "github.com/openshift/api/user/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// IsUserAuthorized checks if the given username is authorized to access Che
// based on the AdvancedAuthorization configuration.
//
// Evaluation order:
//  1. If authz is nil, allow all (return true).
//  2. If username is in DenyUsers, deny immediately.
//  3. If username is in DenyGroups (OpenShift only), deny immediately.
//  4. If AllowUsers or AllowGroups are configured:
//     - Allow if username is in AllowUsers.
//     - Allow if username is in any AllowGroups (OpenShift only).
//     - Otherwise deny.
//  5. If no allow rules configured, allow all.
func IsUserAuthorized(ctx context.Context, clusterAPI chetypes.ClusterAPI, authz *chev2.AdvancedAuthorization, username string) (bool, error) {
	if authz == nil {
		return true, nil
	}

	// Check DenyUsers first.
	if slices.Contains(authz.DenyUsers, username) {
		return false, nil
	}

	// Check DenyGroups (OpenShift only).
	if infrastructure.IsOpenShift() && len(authz.DenyGroups) > 0 {
		denied, err := isUserInGroups(ctx, clusterAPI, username, authz.DenyGroups)
		if err != nil {
			return false, err
		}
		if denied {
			return false, nil
		}
	}

	// Determine if any allow rules exist.
	hasAllowUsers := len(authz.AllowUsers) > 0
	hasAllowGroups := infrastructure.IsOpenShift() && len(authz.AllowGroups) > 0

	if !hasAllowUsers && !hasAllowGroups {
		// No allow rules: permit all.
		return true, nil
	}

	// Check AllowUsers.
	if slices.Contains(authz.AllowUsers, username) {
		return true, nil
	}

	// Check AllowGroups (OpenShift only).
	if hasAllowGroups {
		allowed, err := isUserInGroups(ctx, clusterAPI, username, authz.AllowGroups)
		if err != nil {
			return false, err
		}
		if allowed {
			return true, nil
		}
	}

	return false, nil
}

// isUserInGroups returns true if the given username is a member of any of the
// specified OpenShift groups. It fetches each group by name using the
// NonCachingClient.
func isUserInGroups(ctx context.Context, clusterAPI chetypes.ClusterAPI, username string, groups []string) (bool, error) {
	for _, groupName := range groups {
		group := &userv1.Group{}
		if err := clusterAPI.NonCachingClient.Get(ctx, client.ObjectKey{Name: groupName}, group); err != nil {
			// If the group does not exist, skip it rather than returning an error.
			if client.IgnoreNotFound(err) != nil {
				return false, err
			}
			continue
		}
		if slices.Contains([]string(group.Users), username) {
			return true, nil
		}
	}
	return false, nil
}
