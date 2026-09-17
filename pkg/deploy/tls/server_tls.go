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

	"github.com/go-logr/logr"
	configv1 "github.com/openshift/api/config/v1"
	tlspkg "github.com/openshift/controller-runtime-common/pkg/tls"
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
	profileFetched            bool
}

// BuildServerTLSOptions fetches TLS profile and adherence policy from cluster.
// Returns TLS config functions when adherence policy requires strict compliance.
// Falls back to empty TLSOpts on non-OpenShift, RBAC failures, or legacy adherence policy.
func BuildServerTLSOptions(ctx context.Context, cfg *rest.Config, scheme *k8sruntime.Scheme, log logr.Logger) ServerTLS {
	if !infrastructure.IsOpenShift() {
		return ServerTLS{}
	}

	cl, err := client.New(cfg, client.Options{Scheme: scheme})
	if err != nil {
		log.Error(err, "failed to create client for TLS profile fetch")
		return ServerTLS{}
	}

	apiServer := &configv1.APIServer{}
	if err := cl.Get(ctx, client.ObjectKey{Name: tlspkg.APIServerName}, apiServer); err != nil {
		log.Error(err, "failed to read APIServer/cluster, using Go defaults")
		return ServerTLS{}
	}

	profile, err := tlspkg.GetTLSProfileSpec(apiServer.Spec.TLSSecurityProfile)
	if err != nil {
		log.Error(err, "failed to resolve TLS profile, using Go defaults")
		return ServerTLS{}
	}

	adherence := apiServer.Spec.TLSAdherence

	serverTLS := ServerTLS{
		InitialTLSProfileSpec:     profile,
		InitialTLSAdherencePolicy: adherence,
		profileFetched:            true,
	}

	if shouldHonorClusterTLSProfile(adherence) {
		tlsConfigFn, unsupported := tlspkg.NewTLSConfigFromProfile(profile)
		if len(unsupported) > 0 {
			log.Info("TLS profile contains ciphers unsupported by Go", "unsupported", unsupported)
		}

		serverTLS.TLSOpts = []func(*cryptotls.Config){tlsConfigFn}

		log.Info(
			"Applying cluster TLS profile to the webhook server",
			"minTLSVersion", profile.MinTLSVersion,
			"ciphers", profile.Ciphers,
		)
	} else {
		log.Info("TLS adherence policy does not require strict compliance, using Go default TLS configuration",
			"adherencePolicy", adherence,
		)
	}

	return serverTLS
}

// RegisterSecurityProfileWatcher sets up watcher to restart operator when profile/policy changes.
// Only registers when profile was successfully fetched.
func RegisterSecurityProfileWatcher(mgr manager.Manager, serverTLS ServerTLS, onCancel context.CancelFunc, log logr.Logger) error {
	if !serverTLS.profileFetched {
		return nil
	}

	watcher := &tlspkg.SecurityProfileWatcher{
		Client:                    mgr.GetClient(),
		InitialTLSProfileSpec:     serverTLS.InitialTLSProfileSpec,
		InitialTLSAdherencePolicy: serverTLS.InitialTLSAdherencePolicy,
		OnProfileChange: func(_ context.Context, _, newSpec configv1.TLSProfileSpec) {
			if !shouldHonorClusterTLSProfile(serverTLS.InitialTLSAdherencePolicy) {
				log.V(1).Info("Cluster TLS profile changed but adherence policy is not strict, not restarting")
				return
			}
			log.Info("TLS security profile changed, restarting operator", "minTLSVersion", newSpec.MinTLSVersion)
			onCancel()
		},
		OnAdherencePolicyChange: func(_ context.Context, _, newPolicy configv1.TLSAdherencePolicy) {
			log.Info("TLS adherence policy changed, restarting operator", "adherencePolicy", newPolicy)
			onCancel()
		},
	}

	return watcher.SetupWithManager(mgr)
}

// shouldHonorClusterTLSProfile returns true when tlsAdherence requires strict adherence.
// Unknown values return true for forward compatibility.
func shouldHonorClusterTLSProfile(adherence configv1.TLSAdherencePolicy) bool {
	switch adherence {
	case configv1.TLSAdherencePolicyNoOpinion, configv1.TLSAdherencePolicyLegacyAdheringComponentsOnly:
		return false
	default:
		return true
	}
}
