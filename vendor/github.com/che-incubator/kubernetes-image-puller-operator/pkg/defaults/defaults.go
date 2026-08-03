//
// Copyright (c) 2012-2021 Red Hat, Inc.
// This program and the accompanying materials are made
// available under the terms of the Eclipse Public License 2.0
// which is available at https://www.eclipse.org/legal/epl-2.0/
//
// SPDX-License-Identifier: EPL-2.0
//
// Contributors:
//   Red Hat, Inc. - initial API and implementation
//

package defaults

import corev1 "k8s.io/api/core/v1"

const (
	ConfigMapName    = "k8s-image-puller"
	DeploymentName   = "kubernetes-image-puller"
	ImagePullerImage = "quay.io/eclipse/kubernetes-image-puller:1.1.2"

	AppLabelValue      = "kubernetes-image-puller"
	ContainerName      = "kubernetes-image-puller"
	DaemonSetName      = "kubernetes-image-puller"
	RBACName           = "create-daemonset"
	ServiceAccountName = "k8s-image-puller"

	NonRootUID = int64(65532)
	NonRootGID = int64(65532)
)

var (
	SeccompProfile = corev1.SeccompProfile{Type: corev1.SeccompProfileTypeRuntimeDefault}
)
