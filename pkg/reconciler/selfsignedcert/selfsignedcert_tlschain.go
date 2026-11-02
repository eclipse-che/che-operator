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

package selfsignedcert

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"slices"
	"time"

	"github.com/eclipse-che/che-operator/pkg/common/chetypes"
	"golang.org/x/net/http/httpproxy"
)

const tlsEndpointTimeout = 3 * time.Second

// fetchTLSCertificateChain retrieves the TLS certificates chain from the Che endpoint.
// The endpoint is first reached directly (bypassing the proxy) to ensure the right certificate is
// retrieved; if that fails and a proxy is configured, the request is retried through the proxy.
func (s *SelfSignedCertificateReconciler) fetchTLSCertificateChain(cheCtx *chetypes.CheContext) ([]*x509.Certificate, error) {
	if cheCtx.CheHost == "" {
		return nil, fmt.Errorf("failed to get certificate chain: Che host is not set yet")
	}

	requestURL := "https://" + cheCtx.CheHost

	httpClient := buildHTTPClient()
	certificates, err := s.fetchPeerCertificates(cheCtx.Context, httpClient, requestURL)
	if err == nil {
		return certificates, nil
	}

	if cheCtx.Proxy != nil && cheCtx.Proxy.HttpProxy != "" {
		httpClient = buildHTTPClientWithProxy(cheCtx.Proxy)
		return s.fetchPeerCertificates(cheCtx.Context, httpClient, requestURL)
	}

	return nil, err
}

// fetchPeerCertificates performs a GET request to requestURL and returns the
// certificate chain served on the resulting TLS connection.
func (s *SelfSignedCertificateReconciler) fetchPeerCertificates(
	ctx context.Context,
	httpClient *http.Client,
	requestURL string,
) ([]*x509.Certificate, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request to %s: %w", requestURL, err)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get certificate chain from %s: %w", requestURL, err)
	}

	defer func() {
		if err := resp.Body.Close(); err != nil {
			logger.Error(err, "failed to close response body")
		}
	}()

	if resp.TLS == nil {
		return nil, fmt.Errorf("failed to get TLS connection info from %s: response is not TLS-encrypted", requestURL)
	}

	return resp.TLS.PeerCertificates, nil
}

func containsSelfSignedCertificate(certificateChain []*x509.Certificate) bool {
	return slices.ContainsFunc(
		certificateChain,
		func(cert *x509.Certificate) bool {
			return cert.Subject.String() == cert.Issuer.String()
		},
	)
}

func encodeCertificateChainPEM(certificateChain []*x509.Certificate) []byte {
	var crtBytes []byte

	for i := range certificateChain {
		rawCert := certificateChain[i].Raw
		pemCert := pem.EncodeToMemory(&pem.Block{
			Type:  "CERTIFICATE",
			Bytes: rawCert,
		})
		crtBytes = append(crtBytes, pemCert...)
	}

	return crtBytes
}

func buildHTTPClient() *http.Client {
	return &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
		Timeout: tlsEndpointTimeout,
	}
}

func buildHTTPClientWithProxy(cheProxy *chetypes.Proxy) *http.Client {
	return &http.Client{
		Transport: buildHTTPTransport(cheProxy),
		Timeout:   tlsEndpointTimeout,
	}
}

func buildHTTPTransport(cheProxy *chetypes.Proxy) *http.Transport {
	httpTransport := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}

	config := httpproxy.Config{
		HTTPProxy:  cheProxy.HttpProxy,
		HTTPSProxy: cheProxy.HttpsProxy,
		NoProxy:    cheProxy.NoProxy,
	}
	proxyFunc := config.ProxyFunc()

	httpTransport.Proxy = func(r *http.Request) (*url.URL, error) {
		proxyURL, err := proxyFunc(r.URL)
		if err != nil {
			logger.Error(err, "Failed to get proxy to access TLS endpoint URL", "url", r.URL)
		}
		if proxyURL != nil {
			logger.Info("Using proxy to access TLS endpoint URL", "proxy", proxyURL, "url", r.URL)
		}
		return proxyURL, err
	}

	return httpTransport
}

func isTimeoutError(err error) bool {
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}

	var netErr net.Error
	return errors.As(err, &netErr) && netErr.Timeout()
}
