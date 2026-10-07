// Copyright (c) 2021 Red Hat, Inc.
// This program and the accompanying materials are made
// available under the terms of the Eclipse Public License 2.0
// which is available at https://www.eclipse.org/legal/epl-2.0/
//
// SPDX-License-Identifier: EPL-2.0
//
// Contributors:
//
//	Red Hat, Inc. - initial API and implementation
package header_rewrite_traefik_plugin

import (
	"context"
	"net/http"
)

type Config struct {
	From               string `json:"from,omitempty"`
	To                 string `json:"to,omitempty"`
	Prefix             string `json:"prefix,omitempty"`
	KeepOriginal       bool   `json:"keepOriginal,omitempty"`
	KeepOriginalTarget bool   `json:"keepOriginalTarget,omitempty"`
}

func CreateConfig() *Config {
	return &Config{}
}

type HeaderRewrite struct {
	next   http.Handler
	name   string
	config *Config
}

func New(ctx context.Context, next http.Handler, config *Config, name string) (http.Handler, error) {
	return &HeaderRewrite{
		next: next, config: config, name: name,
	}, nil
}

func (headerRewrite *HeaderRewrite) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	rewriteHeaders(&req.Header, headerRewrite.config)
	headerRewrite.next.ServeHTTP(rw, req)
}

func rewriteHeaders(headers *http.Header, rule *Config) {
	headerValues := headers.Values(rule.From)
	if !rule.KeepOriginalTarget {
		headers.Del(rule.To)
	}
	for _, headerValue := range headerValues {
		if headerValue != "" {
			if len(rule.Prefix) > 0 {
				headerValue = rule.Prefix + headerValue
			}
			headers.Add(rule.To, headerValue)
		}
	}

	if !rule.KeepOriginal {
		headers.Del(rule.From)
	}
}
