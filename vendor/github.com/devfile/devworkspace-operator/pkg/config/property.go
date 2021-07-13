//
// Copyright (c) 2019-2021 Red Hat, Inc.
// This program and the accompanying materials are made
// available under the terms of the Eclipse Public License 2.0
// which is available at https://www.eclipse.org/legal/epl-2.0/
//
// SPDX-License-Identifier: EPL-2.0
//
// Contributors:
//   Red Hat, Inc. - initial API and implementation
//

package config

const (
	// image pull policy that is applied to every container within workspace
	sidecarPullPolicy        = "devworkspace.sidecar.image_pull_policy"
	defaultSidecarPullPolicy = "Always"

	// workspacePVCName config property handles the PVC name that should be created and used for all workspaces within one kubernetes namespace
	workspacePVCName        = "devworkspace.pvc.name"
	defaultWorkspacePVCName = "claim-devworkspace"

	workspacePVCStorageClassName = "devworkspace.pvc.storage_class.name"

	// routingClass defines the default routing class that should be used if user does not specify it explicitly
	routingClass        = "devworkspace.default_routing_class"
	defaultRoutingClass = "basic"

	// RoutingSuffix is the base domain for routes/ingresses created on the cluster. All
	// routes/ingresses will be created with URL http(s)://<unique-to-workspace-part>.<RoutingSuffix>
	// is supposed to be used by embedded routing solvers only
	RoutingSuffix = "devworkspace.routing.cluster_host_suffix"

	experimentalFeaturesEnabled        = "devworkspace.experimental_features_enabled"
	defaultExperimentalFeaturesEnabled = "false"

	devworkspaceIdleTimeout        = "devworkspace.idle_timeout"
	defaultDevWorkspaceIdleTimeout = "15m"

	// Skip Verify for TLS connections
	// It's insecure and should be used only for testing
	tlsInsecureSkipVerify        = "tls.insecure_skip_verify"
	defaultTlsInsecureSkipVerify = "false"
)
