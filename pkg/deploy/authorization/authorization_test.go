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
	"testing"

	chev2 "github.com/eclipse-che/che-operator/api/v2"
	"github.com/stretchr/testify/assert"
)

func TestAllowAllWhenNilConfig(t *testing.T) {
	assert.True(t, IsAuthorized("anyone", nil, nil))
}

func TestAllowAllWhenEmptyRules(t *testing.T) {
	advancedAuthorization := &chev2.AdvancedAuthorization{}
	assert.True(t, IsAuthorized("anyone", nil, advancedAuthorization))
}

func TestDenyUserTakesPrecedence(t *testing.T) {
	advancedAuthorization := &chev2.AdvancedAuthorization{
		AllowUsers: []string{"user1"},
		DenyUsers:  []string{"user1"},
	}
	assert.False(t, IsAuthorized("user1", nil, advancedAuthorization))
}

func TestDenyGroupTakesPrecedence(t *testing.T) {
	advancedAuthorization := &chev2.AdvancedAuthorization{
		AllowGroups: []string{"devs"},
		DenyGroups:  []string{"devs"},
	}
	assert.False(t, IsAuthorized("user1", []string{"devs"}, advancedAuthorization))
}

func TestAllowUserExplicitly(t *testing.T) {
	advancedAuthorization := &chev2.AdvancedAuthorization{
		AllowUsers: []string{"user1"},
	}
	assert.True(t, IsAuthorized("user1", nil, advancedAuthorization))
	assert.False(t, IsAuthorized("user2", nil, advancedAuthorization))
}

func TestAllowGroupExplicitly(t *testing.T) {
	advancedAuthorization := &chev2.AdvancedAuthorization{
		AllowGroups: []string{"devs"},
	}
	assert.True(t, IsAuthorized("user1", []string{"devs"}, advancedAuthorization))
	assert.False(t, IsAuthorized("user2", []string{"other"}, advancedAuthorization))
}

func TestDenyUserOnly(t *testing.T) {
	advancedAuthorization := &chev2.AdvancedAuthorization{
		DenyUsers: []string{"blocked"},
	}
	assert.False(t, IsAuthorized("blocked", nil, advancedAuthorization))
	assert.True(t, IsAuthorized("allowed", nil, advancedAuthorization))
}

func TestDenyGroupOnly(t *testing.T) {
	advancedAuthorization := &chev2.AdvancedAuthorization{
		DenyGroups: []string{"banned"},
	}
	assert.False(t, IsAuthorized("user1", []string{"banned"}, advancedAuthorization))
	assert.True(t, IsAuthorized("user1", []string{"ok"}, advancedAuthorization))
}

func TestEmptyUsername(t *testing.T) {
	advancedAuthorization := &chev2.AdvancedAuthorization{
		AllowUsers: []string{"user1"},
	}
	assert.False(t, IsAuthorized("", nil, advancedAuthorization))
}

func TestMultipleGroups(t *testing.T) {
	advancedAuthorization := &chev2.AdvancedAuthorization{
		AllowGroups: []string{"admins"},
	}
	assert.True(t, IsAuthorized("user1", []string{"devs", "admins"}, advancedAuthorization))
}
