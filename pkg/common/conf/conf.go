//
// Copyright (c) 2025 Red Hat, Inc.
// This program and the accompanying materials are made
// available under the terms of the Eclipse Public License 2.0
// which is available at https://www.eclipse.org/legal/epl-2.0/
//
// SPDX-License-Identifier: EPL-2.0
//
// Contributors:
//   Red Hat, Inc. - initial API and implementation
//

package conf

import "github.com/eclipse-che/che-operator/pkg/common/utils"

var (
	// since 7.98.0
	// The namespace where the custom certificate ConfigMap is stored
	// See https://docs.openshift.com/container-platform/latest/security/certificates/updating-ca-bundle.html
	CertificatesOpenShiftConfigNamespaceName = &CheOperatorConf{
		EnvName:      "CHE_OPERATOR_CERTIFICATES_OPENSHIFT_CONFIG_NAMESPACE_NAME",
		defaultValue: "openshift-config",
	}

	// since 7.98.0
	// The key name containing the custom certificate in the ConfigMap
	// See https://docs.openshift.com/container-platform/latest/security/certificates/updating-ca-bundle.html
	CertificatesOpenShiftCustomCertificateConfigMapKeyName = &CheOperatorConf{
		EnvName:      "CHE_OPERATOR_CERTIFICATES_OPENSHIFT_CUSTOM_CERTIFICATE_CONFIGMAP_KEY_NAME",
		defaultValue: "ca-bundle.crt",
	}

	// since 7.98.0
	// Introduce a new behavior to sync only custom certificates
	// "false" means previous behaviour when the whole OpenShift certificates bundle was synced
	// by adding "config.openshift.io/inject-trusted-cabundle=true" label
	// https://docs.openshift.com/container-platform/latest/networking/configuring-a-custom-pki.html#certificate-injection-using-operators_configuring-a-custom-pki
	CertificatesSyncCustomOpenShiftCertificateOnly = &CheOperatorConf{
		EnvName:      "CHE_OPERATOR_CERTIFICATES_SYNC_CUSTOM_OPENSHIFT_CERTIFICATE_ONLY",
		defaultValue: "true",
	}
)

type CheOperatorConf struct {
	EnvName      string
	defaultValue string
}

func Get(conf *CheOperatorConf) string {
	return utils.GetEnvOrDefault(conf.EnvName, conf.defaultValue)
}
