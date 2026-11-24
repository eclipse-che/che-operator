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

package tls

import (
	"context"
	cryptotls "crypto/tls"
	"testing"

	"github.com/eclipse-che/che-operator/pkg/common/infrastructure"
	"github.com/eclipse-che/che-operator/pkg/common/test"
	configv1 "github.com/openshift/api/config/v1"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
)

func TestBuildServerTLSOptions_APIServerAbsent(t *testing.T) {
	test.StartOn(infrastructure.OpenShiftV4)

	cheCtx := test.NewCheContextBuilder().Build()
	log := ctrl.Log.WithName("test")

	got, err := buildServerTLSOptions(context.Background(), cheCtx.ClusterAPI.Client, log)

	assert.NoError(t, err)

	assert.NotEmpty(t, got.TLSOpts, "should fall back to library-go default TLS profile")

	cfg := &cryptotls.Config{}
	for _, opt := range got.TLSOpts {
		opt(cfg)
	}

	assert.Greater(t, cfg.MinVersion, uint16(0), "default profile should set a minimum TLS version")
}

func TestBuildServerTLSOptions_StrictWithModernProfile(t *testing.T) {
	test.StartOn(infrastructure.OpenShiftV4)

	apiServer := &configv1.APIServer{
		ObjectMeta: metav1.ObjectMeta{
			Name: "cluster",
		},
		Spec: configv1.APIServerSpec{
			TLSSecurityProfile: &configv1.TLSSecurityProfile{
				Type: configv1.TLSProfileModernType,
			},
			TLSAdherence: configv1.TLSAdherencePolicyStrictAllComponents,
		},
	}

	cheCtx := test.NewCheContextBuilder().WithObjects(apiServer).Build()
	log := ctrl.Log.WithName("test")

	got, err := buildServerTLSOptions(context.Background(), cheCtx.ClusterAPI.Client, log)

	assert.NoError(t, err)

	assert.NotEmpty(t, got.TLSOpts)

	cfg := &cryptotls.Config{}
	for _, opt := range got.TLSOpts {
		opt(cfg)
	}

	assert.Equal(t, uint16(cryptotls.VersionTLS13), cfg.MinVersion)
	assert.Nil(t, cfg.CipherSuites, "TLS 1.3 does not allow configuring cipher suites")
}

func TestBuildServerTLSOptions_StrictWithOldProfile(t *testing.T) {
	apiServer := &configv1.APIServer{
		ObjectMeta: metav1.ObjectMeta{
			Name: "cluster",
		},
		Spec: configv1.APIServerSpec{
			TLSSecurityProfile: &configv1.TLSSecurityProfile{
				Type: configv1.TLSProfileOldType,
			},
			TLSAdherence: configv1.TLSAdherencePolicyStrictAllComponents,
		},
	}

	test.StartOn(infrastructure.OpenShiftV4)

	cheCtx := test.NewCheContextBuilder().WithObjects(apiServer).Build()
	log := ctrl.Log.WithName("test")

	got, err := buildServerTLSOptions(context.Background(), cheCtx.ClusterAPI.Client, log)

	assert.NoError(t, err)

	assert.NotEmpty(t, got.TLSOpts)

	cfg := &cryptotls.Config{}
	for _, opt := range got.TLSOpts {
		opt(cfg)
	}

	assert.Equal(t, uint16(cryptotls.VersionTLS10), cfg.MinVersion)
	assert.NotEmpty(t, cfg.CipherSuites)
}

func TestBuildServerTLSOptions_NoOpinionUsesDefaultProfile(t *testing.T) {
	test.StartOn(infrastructure.OpenShiftV4)

	apiServer := &configv1.APIServer{
		ObjectMeta: metav1.ObjectMeta{
			Name: "cluster",
		},
		Spec: configv1.APIServerSpec{
			TLSAdherence: configv1.TLSAdherencePolicyNoOpinion,
		},
	}

	cheCtx := test.NewCheContextBuilder().WithObjects(apiServer).Build()
	log := ctrl.Log.WithName("test")

	got, err := buildServerTLSOptions(context.Background(), cheCtx.ClusterAPI.Client, log)

	assert.NoError(t, err)

	assert.NotEmpty(t, got.TLSOpts, "should apply library-go default TLS profile")

	cfg := &cryptotls.Config{}
	for _, opt := range got.TLSOpts {
		opt(cfg)
	}

	assert.Greater(t, cfg.MinVersion, uint16(0), "default profile should set a minimum TLS version")
}
