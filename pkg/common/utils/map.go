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

package utils

import (
	"maps"
	"slices"
	"strings"
)

func NonNilMap[K comparable, V any](m map[K]V) map[K]V {
	if m == nil {
		return make(map[K]V)
	}
	return m
}

func ParseMap(src string) map[string]string {
	if src == "" {
		return nil
	}

	m := make(map[string]string)
	for item := range strings.SplitSeq(src, ",") {
		key, value, ok := strings.Cut(item, "=")
		if !ok {
			continue
		}

		key, value = strings.TrimSpace(key), strings.TrimSpace(value)
		if key != "" {
			m[key] = value
		}
	}

	return m
}

func FormatMap(m map[string]string) string {
	if len(m) == 0 {
		return ""
	}

	var b strings.Builder
	for _, key := range slices.Sorted(maps.Keys(m)) {
		if b.Len() > 0 {
			b.WriteByte(',')
		}

		b.WriteString(key)
		b.WriteByte('=')
		b.WriteString(m[key])
	}

	return b.String()
}

// MergeMapsMissingKeys merges src into dst by adding only key-value pairs whose keys
// are not yet present in dst. Values already in dst are never overwritten.
// It returns dst (created if nil), so it can be used inline.
func MergeMapsMissingKeys[K comparable, V any](dst, src map[K]V) map[K]V {
	if dst == nil {
		dst = make(map[K]V, len(src))
	}

	for k, v := range src {
		if _, exists := dst[k]; !exists {
			dst[k] = v
		}
	}

	return dst
}
