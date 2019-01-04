//
// Copyright (c) 2012-2018 Red Hat, Inc.
// This program and the accompanying materials are made
// available under the terms of the Eclipse Public License 2.0
// which is available at https://www.eclipse.org/legal/epl-2.0/
//
// SPDX-License-Identifier: EPL-2.0
//
// Contributors:
//   Red Hat, Inc. - initial API and implementation
//
package util

import "encoding/base64"

// GetSelfSignedCert get content of provided self signed certificate
func GetSelfSignedCert() (certContent []byte) {
	base64Cert := GetEnv("CHE_SELF__SIGNED__CERT","")
	certContent, _ = base64.StdEncoding.DecodeString(base64Cert)
	return certContent
}