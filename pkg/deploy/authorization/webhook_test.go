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
	"context"
	"net/http"
	"testing"

	chev2 "github.com/eclipse-che/che-operator/api/v2"
	"github.com/eclipse-che/che-operator/pkg/common/test"
	"github.com/stretchr/testify/assert"
	admissionv1 "k8s.io/api/admission/v1"
	authenticationv1 "k8s.io/api/authentication/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

func TestHandleAllowedWhenNoAdvancedAuthorization(t *testing.T) {
	ctx := test.NewCtxBuilder().Build()
	webhook := NewAuthorizationWebhook(ctx.ClusterAPI.Client)

	response := webhook.Handle(context.Background(), newAdmissionRequest("user1", nil))

	assert.True(t, response.Allowed)
}

func TestHandleAllowedWhenUserInAllowList(t *testing.T) {
	ctx := test.NewCtxBuilder().WithCheCluster(&chev2.CheCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "eclipse-che",
			Namespace: "eclipse-che",
		},
		Spec: chev2.CheClusterSpec{
			Networking: chev2.CheClusterSpecNetworking{
				Auth: chev2.Auth{
					AdvancedAuthorization: &chev2.AdvancedAuthorization{
						AllowUsers: []string{"user1"},
					},
				},
			},
		},
	}).Build()
	webhook := NewAuthorizationWebhook(ctx.ClusterAPI.Client)

	response := webhook.Handle(context.Background(), newAdmissionRequest("user1", nil))

	assert.True(t, response.Allowed)
}

func TestHandleDeniedWhenUserNotInAllowList(t *testing.T) {
	ctx := test.NewCtxBuilder().WithCheCluster(&chev2.CheCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "eclipse-che",
			Namespace: "eclipse-che",
		},
		Spec: chev2.CheClusterSpec{
			Networking: chev2.CheClusterSpecNetworking{
				Auth: chev2.Auth{
					AdvancedAuthorization: &chev2.AdvancedAuthorization{
						AllowUsers: []string{"user1"},
					},
				},
			},
		},
	}).Build()
	webhook := NewAuthorizationWebhook(ctx.ClusterAPI.Client)

	response := webhook.Handle(context.Background(), newAdmissionRequest("user2", nil))

	assert.False(t, response.Allowed)
	assert.Equal(t, int32(http.StatusForbidden), response.Result.Code)
}

func TestHandleDeniedWhenUserInDenyList(t *testing.T) {
	ctx := test.NewCtxBuilder().WithCheCluster(&chev2.CheCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "eclipse-che",
			Namespace: "eclipse-che",
		},
		Spec: chev2.CheClusterSpec{
			Networking: chev2.CheClusterSpecNetworking{
				Auth: chev2.Auth{
					AdvancedAuthorization: &chev2.AdvancedAuthorization{
						DenyUsers: []string{"blocked"},
					},
				},
			},
		},
	}).Build()
	webhook := NewAuthorizationWebhook(ctx.ClusterAPI.Client)

	response := webhook.Handle(context.Background(), newAdmissionRequest("blocked", nil))

	assert.False(t, response.Allowed)
	assert.Equal(t, int32(http.StatusForbidden), response.Result.Code)
}

func TestHandleAllowedWhenUserGroupInAllowList(t *testing.T) {
	ctx := test.NewCtxBuilder().WithCheCluster(&chev2.CheCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "eclipse-che",
			Namespace: "eclipse-che",
		},
		Spec: chev2.CheClusterSpec{
			Networking: chev2.CheClusterSpecNetworking{
				Auth: chev2.Auth{
					AdvancedAuthorization: &chev2.AdvancedAuthorization{
						AllowGroups: []string{"devs"},
					},
				},
			},
		},
	}).Build()
	webhook := NewAuthorizationWebhook(ctx.ClusterAPI.Client)

	response := webhook.Handle(context.Background(), newAdmissionRequest("user1", []string{"devs"}))

	assert.True(t, response.Allowed)
}

func TestHandleErrorWhenCheClusterNotFound(t *testing.T) {
	ctx := test.NewCtxBuilder().WithCheCluster(nil).Build()
	webhook := NewAuthorizationWebhook(ctx.ClusterAPI.Client)

	response := webhook.Handle(context.Background(), newAdmissionRequest("user1", nil))

	assert.False(t, response.Allowed)
	assert.Equal(t, int32(http.StatusInternalServerError), response.Result.Code)
}

func newAdmissionRequest(username string, groups []string) admission.Request {
	return admission.Request{
		AdmissionRequest: admissionv1.AdmissionRequest{
			UserInfo: authenticationv1.UserInfo{
				Username: username,
				Groups:   groups,
			},
		},
	}
}
