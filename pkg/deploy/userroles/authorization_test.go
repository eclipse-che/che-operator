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
	"testing"

	chev2 "github.com/eclipse-che/che-operator/api/v2"
	"github.com/eclipse-che/che-operator/pkg/common/chetypes"
	"github.com/eclipse-che/che-operator/pkg/common/infrastructure"
	userv1 "github.com/openshift/api/user/v1"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func buildClusterAPIWithGroups(groups ...*userv1.Group) chetypes.ClusterAPI {
	scheme := runtime.NewScheme()
	_ = userv1.Install(scheme)

	objs := make([]client.Object, 0, len(groups))
	for _, g := range groups {
		objs = append(objs, g)
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(objs...).
		Build()

	return chetypes.ClusterAPI{
		NonCachingClient: fakeClient,
		Scheme:           scheme,
	}
}

func makeGroup(name string, users ...string) *userv1.Group {
	return &userv1.Group{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Users:      userv1.OptionalNames(users),
	}
}

// TestIsUserAuthorized_NilAuthz verifies that a nil AdvancedAuthorization
// grants access to every user.
func TestIsUserAuthorized_NilAuthz(t *testing.T) {
	clusterAPI := buildClusterAPIWithGroups()
	ok, err := IsUserAuthorized(context.TODO(), clusterAPI, nil, "alice")
	assert.NoError(t, err)
	assert.True(t, ok)
}

// TestIsUserAuthorized_EmptyAuthz verifies that an empty AdvancedAuthorization
// (no allow/deny lists) grants access to everyone.
func TestIsUserAuthorized_EmptyAuthz(t *testing.T) {
	clusterAPI := buildClusterAPIWithGroups()
	authz := &chev2.AdvancedAuthorization{}
	ok, err := IsUserAuthorized(context.TODO(), clusterAPI, authz, "alice")
	assert.NoError(t, err)
	assert.True(t, ok)
}

// TestIsUserAuthorized_AllowUsers verifies that a user in AllowUsers is allowed.
func TestIsUserAuthorized_AllowUsers(t *testing.T) {
	clusterAPI := buildClusterAPIWithGroups()
	authz := &chev2.AdvancedAuthorization{
		AllowUsers: []string{"alice", "bob"},
	}

	ok, err := IsUserAuthorized(context.TODO(), clusterAPI, authz, "alice")
	assert.NoError(t, err)
	assert.True(t, ok)
}

// TestIsUserAuthorized_AllowUsers_NotInList verifies that a user not in AllowUsers
// is denied when AllowUsers is configured.
func TestIsUserAuthorized_AllowUsers_NotInList(t *testing.T) {
	clusterAPI := buildClusterAPIWithGroups()
	authz := &chev2.AdvancedAuthorization{
		AllowUsers: []string{"alice", "bob"},
	}

	ok, err := IsUserAuthorized(context.TODO(), clusterAPI, authz, "carol")
	assert.NoError(t, err)
	assert.False(t, ok)
}

// TestIsUserAuthorized_DenyUsers verifies that a user in DenyUsers is denied.
func TestIsUserAuthorized_DenyUsers(t *testing.T) {
	clusterAPI := buildClusterAPIWithGroups()
	authz := &chev2.AdvancedAuthorization{
		DenyUsers: []string{"mallory"},
	}

	ok, err := IsUserAuthorized(context.TODO(), clusterAPI, authz, "mallory")
	assert.NoError(t, err)
	assert.False(t, ok)
}

// TestIsUserAuthorized_DenyUsers_NotInList verifies that a user not in DenyUsers
// is allowed when no allow rules are configured.
func TestIsUserAuthorized_DenyUsers_NotInList(t *testing.T) {
	clusterAPI := buildClusterAPIWithGroups()
	authz := &chev2.AdvancedAuthorization{
		DenyUsers: []string{"mallory"},
	}

	ok, err := IsUserAuthorized(context.TODO(), clusterAPI, authz, "alice")
	assert.NoError(t, err)
	assert.True(t, ok)
}

// TestIsUserAuthorized_DenyOverridesAllow verifies that DenyUsers takes
// precedence over AllowUsers.
func TestIsUserAuthorized_DenyOverridesAllow(t *testing.T) {
	clusterAPI := buildClusterAPIWithGroups()
	authz := &chev2.AdvancedAuthorization{
		AllowUsers: []string{"alice", "mallory"},
		DenyUsers:  []string{"mallory"},
	}

	ok, err := IsUserAuthorized(context.TODO(), clusterAPI, authz, "mallory")
	assert.NoError(t, err)
	assert.False(t, ok)

	ok, err = IsUserAuthorized(context.TODO(), clusterAPI, authz, "alice")
	assert.NoError(t, err)
	assert.True(t, ok)
}

// TestIsUserAuthorized_AllowGroups_OpenShift verifies that a user in an
// allowed OpenShift group is granted access.
func TestIsUserAuthorized_AllowGroups_OpenShift(t *testing.T) {
	infrastructure.InitializeForTesting(infrastructure.OpenShiftV4)

	devGroup := makeGroup("developers", "alice", "bob")
	clusterAPI := buildClusterAPIWithGroups(devGroup)

	authz := &chev2.AdvancedAuthorization{
		AllowGroups: []string{"developers"},
	}

	ok, err := IsUserAuthorized(context.TODO(), clusterAPI, authz, "alice")
	assert.NoError(t, err)
	assert.True(t, ok)
}

// TestIsUserAuthorized_AllowGroups_UserNotInGroup verifies that a user not
// in any allowed group is denied when only AllowGroups is configured.
func TestIsUserAuthorized_AllowGroups_UserNotInGroup(t *testing.T) {
	infrastructure.InitializeForTesting(infrastructure.OpenShiftV4)

	devGroup := makeGroup("developers", "alice", "bob")
	clusterAPI := buildClusterAPIWithGroups(devGroup)

	authz := &chev2.AdvancedAuthorization{
		AllowGroups: []string{"developers"},
	}

	ok, err := IsUserAuthorized(context.TODO(), clusterAPI, authz, "carol")
	assert.NoError(t, err)
	assert.False(t, ok)
}

// TestIsUserAuthorized_DenyGroups_OpenShift verifies that a user in a denied
// OpenShift group is blocked.
func TestIsUserAuthorized_DenyGroups_OpenShift(t *testing.T) {
	infrastructure.InitializeForTesting(infrastructure.OpenShiftV4)

	bannedGroup := makeGroup("banned", "mallory")
	clusterAPI := buildClusterAPIWithGroups(bannedGroup)

	authz := &chev2.AdvancedAuthorization{
		DenyGroups: []string{"banned"},
	}

	ok, err := IsUserAuthorized(context.TODO(), clusterAPI, authz, "mallory")
	assert.NoError(t, err)
	assert.False(t, ok)
}

// TestIsUserAuthorized_DenyGroups_UserNotInGroup verifies that a user not in
// any denied group is allowed when no allow rules are configured.
func TestIsUserAuthorized_DenyGroups_UserNotInGroup(t *testing.T) {
	infrastructure.InitializeForTesting(infrastructure.OpenShiftV4)

	bannedGroup := makeGroup("banned", "mallory")
	clusterAPI := buildClusterAPIWithGroups(bannedGroup)

	authz := &chev2.AdvancedAuthorization{
		DenyGroups: []string{"banned"},
	}

	ok, err := IsUserAuthorized(context.TODO(), clusterAPI, authz, "alice")
	assert.NoError(t, err)
	assert.True(t, ok)
}

// TestIsUserAuthorized_AllowGroups_Kubernetes verifies that on plain
// Kubernetes, AllowGroups is a no-op and access is granted (if no AllowUsers).
func TestIsUserAuthorized_AllowGroups_Kubernetes(t *testing.T) {
	infrastructure.InitializeForTesting(infrastructure.Kubernetes)
	defer infrastructure.InitializeForTesting(infrastructure.OpenShiftV4)

	clusterAPI := buildClusterAPIWithGroups()

	authz := &chev2.AdvancedAuthorization{
		AllowGroups: []string{"developers"},
	}

	// On Kubernetes there are no group checks, so with no AllowUsers configured
	// and no effective allow rules, all users should be permitted.
	ok, err := IsUserAuthorized(context.TODO(), clusterAPI, authz, "alice")
	assert.NoError(t, err)
	assert.True(t, ok)
}

// TestIsUserAuthorized_GroupNotFound verifies that a missing group is silently
// skipped (no error, no access granted).
func TestIsUserAuthorized_GroupNotFound(t *testing.T) {
	infrastructure.InitializeForTesting(infrastructure.OpenShiftV4)

	clusterAPI := buildClusterAPIWithGroups() // no groups pre-created

	authz := &chev2.AdvancedAuthorization{
		AllowGroups: []string{"nonexistent"},
	}

	ok, err := IsUserAuthorized(context.TODO(), clusterAPI, authz, "alice")
	assert.NoError(t, err)
	assert.False(t, ok)
}
