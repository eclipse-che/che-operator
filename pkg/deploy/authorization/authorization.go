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

package authorization

import (
	"slices"

	chev2 "github.com/eclipse-che/che-operator/api/v2"
)

func IsAuthorized(username string, groups []string, advancedAuthorization *chev2.AdvancedAuthorization) bool {
	if advancedAuthorization == nil {
		return true
	}

	if username == "" {
		return false
	}

	if slices.Contains(advancedAuthorization.DenyUsers, username) {
		return false
	}

	if matchesAnyGroup(advancedAuthorization.DenyGroups, groups) {
		return false
	}

	if len(advancedAuthorization.AllowUsers) == 0 && len(advancedAuthorization.AllowGroups) == 0 {
		return true
	}

	if slices.Contains(advancedAuthorization.AllowUsers, username) {
		return true
	}

	if matchesAnyGroup(advancedAuthorization.AllowGroups, groups) {
		return true
	}

	return false
}

func matchesAnyGroup(ruleGroups []string, userGroups []string) bool {
	for _, ruleGroup := range ruleGroups {
		if slices.Contains(userGroups, ruleGroup) {
			return true
		}
	}

	return false
}
