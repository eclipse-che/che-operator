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
	"fmt"

	"github.com/go-logr/logr"
	configv1 "github.com/openshift/api/config/v1"
	tlspkg "github.com/openshift/controller-runtime-common/pkg/tls"
	libgocrypto "github.com/openshift/library-go/pkg/crypto"
	k8sruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/eclipse-che/che-operator/pkg/common/infrastructure"
)

// ServerTLS holds TLS options and initial profile/policy for watcher
type ServerTLS struct {
	TLSOpts                   []func(*cryptotls.Config)
	InitialTLSProfileSpec     configv1.TLSProfileSpec
	InitialTLSAdherencePolicy configv1.TLSAdherencePolicy
}

// BuildServerTLSOptions fetches TLS profile and adherence policy from cluster.
// Returns TLS config functions when adherence policy requires strict compliance.
// Falls back to the library-go default TLS profile on transient API server errors
// or when adherence policy does not require strict compliance.
// Returns empty ServerTLS on non-OpenShift clusters.
func BuildServerTLSOptions(ctx context.Context, cfg *rest.Config, scheme *k8sruntime.Scheme, log logr.Logger) (ServerTLS, error) {
	if !infrastructure.IsOpenShift() {
		return ServerTLS{}, nil
	}

	cl, err := client.New(cfg, client.Options{Scheme: scheme})
	if err != nil {
		return ServerTLS{}, fmt.Errorf("failed to create client for TLS profile fetch: %w", err)
	}

	return buildServerTLSOptions(ctx, cl, log)
}

func buildServerTLSOptions(ctx context.Context, cl client.Client, log logr.Logger) (ServerTLS, error) {
	var profile configv1.TLSProfileSpec
	var adherence configv1.TLSAdherencePolicy

	apiServer := &configv1.APIServer{}
	if err := cl.Get(ctx, client.ObjectKey{Name: tlspkg.APIServerName}, apiServer); err != nil {
		log.Error(err, "failed to read APIServer/cluster, falling back to library-go default TLS profile")
	} else if p, err := tlspkg.GetTLSProfileSpec(apiServer.Spec.TLSSecurityProfile); err != nil {
		log.Error(err, "failed to resolve TLS profile spec, falling back to library-go default TLS profile")
	} else {
		profile = p
		adherence = apiServer.Spec.TLSAdherence
	}

	serverTLS := ServerTLS{
		InitialTLSProfileSpec:     profile,
		InitialTLSAdherencePolicy: adherence,
	}

	if libgocrypto.ShouldHonorClusterTLSProfile(adherence) {
		tlsConfigFn, unsupported := tlspkg.NewTLSConfigFromProfile(profile)
		if len(unsupported) > 0 {
			log.Info("TLS profile contains ciphers unsupported by Go", "unsupported", unsupported)
		}

		if len(profile.Ciphers) > 0 && len(unsupported) == len(profile.Ciphers) {
			log.Error(nil, "no ciphers from the cluster TLS profile are supported by Go; server will use library-go defaults, which may not satisfy tlsAdherence",
				"profileCiphers", profile.Ciphers)
		}

		serverTLS.TLSOpts = []func(*cryptotls.Config){tlsConfigFn}

		log.Info(
			"Applying cluster TLS profile to the webhook server",
			"minTLSVersion", profile.MinTLSVersion,
		)
		log.V(1).Info("TLS cipher list from cluster profile", "ciphers", profile.Ciphers)
	} else {
		defaultProfile := *configv1.TLSProfiles[libgocrypto.DefaultTLSProfileType]
		defaultTLSConfigFn, unsupported := tlspkg.NewTLSConfigFromProfile(defaultProfile)
		if len(unsupported) > 0 {
			log.Info("Default TLS profile contains ciphers unsupported by Go", "unsupported", unsupported)
		}

		serverTLS.TLSOpts = []func(*cryptotls.Config){defaultTLSConfigFn}

		log.Info("Using library-go default TLS profile",
			"minTLSVersion", defaultProfile.MinTLSVersion,
			"adherencePolicy", adherence,
		)
	}

	return serverTLS, nil
}

// RegisterSecurityProfileWatcher sets up watcher to restart operator when profile/policy changes.
// Always registers on OpenShift so that changes (or late availability) trigger a restart.
func RegisterSecurityProfileWatcher(mgr manager.Manager, serverTLS ServerTLS, onCancel context.CancelFunc, log logr.Logger) error {
	watcher := &tlspkg.SecurityProfileWatcher{
		Client:                    mgr.GetClient(),
		InitialTLSProfileSpec:     serverTLS.InitialTLSProfileSpec,
		InitialTLSAdherencePolicy: serverTLS.InitialTLSAdherencePolicy,
		OnProfileChange: func(_ context.Context, _, newSpec configv1.TLSProfileSpec) {
			if !libgocrypto.ShouldHonorClusterTLSProfile(serverTLS.InitialTLSAdherencePolicy) {
				log.V(1).Info("Cluster TLS profile changed but adherence policy is not strict, not restarting")
				return
			}

			log.V(1).Info("TLS security profile changed, restarting operator", "minTLSVersion", newSpec.MinTLSVersion)
			onCancel()
		},
		OnAdherencePolicyChange: func(_ context.Context, _, newPolicy configv1.TLSAdherencePolicy) {
			log.V(1).Info("TLS adherence policy changed, restarting operator", "adherencePolicy", newPolicy)
			onCancel()
		},
	}

	return watcher.SetupWithManager(mgr)
}
