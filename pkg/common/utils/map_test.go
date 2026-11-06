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
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestParseMap(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want map[string]string
	}{
		{
			name: "empty string",
			src:  "",
			want: nil,
		},
		{
			name: "single pair",
			src:  "key=value",
			want: map[string]string{"key": "value"},
		},
		{
			name: "multiple pairs",
			src:  "key1=value1,key2=value2",
			want: map[string]string{"key1": "value1", "key2": "value2"},
		},
		{
			name: "trims spaces around key and value",
			src:  " key1 = value1 ,  key2=value2  ",
			want: map[string]string{"key1": "value1", "key2": "value2"},
		},
		{
			name: "skips items without separator",
			src:  "key1=value1,invalid,key2=value2",
			want: map[string]string{"key1": "value1", "key2": "value2"},
		},
		{
			name: "skips empty key or value",
			src:  "=value1,key2=,=,key3=value3",
			want: map[string]string{"key3": "value3"},
		},
		{
			name: "value containing separator",
			src:  "key=a=b",
			want: map[string]string{"key": "a=b"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseMap(tt.src)
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("ParseMap(%q) mismatch (-want +got):\n%s", tt.src, diff)
			}
		})
	}
}

func TestFormatMap(t *testing.T) {
	tests := []struct {
		name string
		m    map[string]string
		want string
	}{
		{
			name: "nil map",
			m:    nil,
			want: "",
		},
		{
			name: "empty map",
			m:    map[string]string{},
			want: "",
		},
		{
			name: "single pair",
			m:    map[string]string{"key": "value"},
			want: "key=value",
		},
		{
			name: "keys are sorted",
			m:    map[string]string{"b": "2", "a": "1", "c": "3"},
			want: "a=1,b=2,c=3",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatMap(tt.m)
			if got != tt.want {
				t.Errorf("FormatMap(%v) = %q, want %q", tt.m, got, tt.want)
			}
		})
	}
}
