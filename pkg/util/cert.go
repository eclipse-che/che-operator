package util

import "encoding/base64"

// GetSelfSignedCert get content of provided self signed certificate
func GetSelfSignedCert() (certContent []byte) {
	base64Cert := GetEnv(SelfSignedCert,"")
	certContent, _ = base64.StdEncoding.DecodeString(base64Cert)
	return certContent
}