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
			name: "don't skips empty value",
			src:  "=value1,key2=,=,key3=value3",
			want: map[string]string{"key2": "", "key3": "value3"},
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
		{
			name: "with empty values",
			m:    map[string]string{"a": "", "b": "2", "c": ""},
			want: "a=,b=2,c=",
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

func TestContainsEntries(t *testing.T) {
	tests := []struct {
		name     string
		actual   map[string]string
		expected map[string]string
		want     bool
	}{
		{
			name:     "nil maps",
			actual:   nil,
			expected: nil,
			want:     true,
		},
		{
			name:     "empty maps",
			actual:   map[string]string{},
			expected: map[string]string{},
			want:     true,
		},
		{
			name:     "all entries match",
			actual:   map[string]string{"a": "1", "b": "2"},
			expected: map[string]string{"a": "1", "b": "2"},
			want:     true,
		},
		{
			name:     "actual is superset",
			actual:   map[string]string{"a": "1", "b": "2", "c": "3"},
			expected: map[string]string{"a": "1"},
			want:     true,
		},
		{
			name:     "missing key with non-zero expected value",
			actual:   map[string]string{"a": "1"},
			expected: map[string]string{"b": "2"},
			want:     false,
		},
		{
			name:     "missing key with zero expected value",
			actual:   map[string]string{"a": "1"},
			expected: map[string]string{"b": ""},
			want:     false,
		},
		{
			name:     "value mismatch",
			actual:   map[string]string{"a": "1"},
			expected: map[string]string{"a": "2"},
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ContainsEntries(tt.actual, tt.expected)
			if got != tt.want {
				t.Errorf("ContainsEntries(%v, %v) = %v, want %v", tt.actual, tt.expected, got, tt.want)
			}
		})
	}
}

func TestFilterMap(t *testing.T) {
	hasPrefixA := func(key string, _ string) bool {
		return len(key) > 0 && key[0] == 'a'
	}

	tests := []struct {
		name  string
		input map[string]string
		want  map[string]string
	}{
		{
			name:  "nil input",
			input: nil,
			want:  map[string]string{},
		},
		{
			name:  "empty input",
			input: map[string]string{},
			want:  map[string]string{},
		},
		{
			name:  "no entries match",
			input: map[string]string{"b": "1", "c": "2"},
			want:  map[string]string{},
		},
		{
			name:  "some entries match",
			input: map[string]string{"a1": "1", "b": "2", "a2": "3"},
			want:  map[string]string{"a1": "1", "a2": "3"},
		},
		{
			name:  "all entries match",
			input: map[string]string{"a1": "1", "a2": "2"},
			want:  map[string]string{"a1": "1", "a2": "2"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FilterMap(tt.input, hasPrefixA)

			if got == nil {
				t.Fatalf("FilterMap(%v) returned nil, want non-nil map", tt.input)
			}

			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("FilterMap(%v) mismatch (-want +got):\n%s", tt.input, diff)
			}
		})
	}
}
