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
)

func TestFind(t *testing.T) {
	type item struct {
		name  string
		value int
	}

	tests := []struct {
		name      string
		slice     []item
		predicate func(item) bool
		wantFound bool
		want      item
	}{
		{
			name:      "nil slice",
			slice:     nil,
			predicate: func(i item) bool { return true },
			wantFound: false,
		},
		{
			name:      "empty slice",
			slice:     []item{},
			predicate: func(i item) bool { return true },
			wantFound: false,
		},
		{
			name:      "no match",
			slice:     []item{{name: "a", value: 1}},
			predicate: func(i item) bool { return i.name == "b" },
			wantFound: false,
		},
		{
			name:      "match",
			slice:     []item{{name: "a", value: 1}, {name: "b", value: 2}},
			predicate: func(i item) bool { return i.name == "b" },
			wantFound: true,
			want:      item{name: "b", value: 2},
		},
		{
			name:      "first match wins",
			slice:     []item{{name: "a", value: 1}, {name: "a", value: 2}},
			predicate: func(i item) bool { return i.name == "a" },
			wantFound: true,
			want:      item{name: "a", value: 1},
		},
		{
			name:      "zero-valued element match",
			slice:     []item{{name: "a", value: 1}, {}},
			predicate: func(i item) bool { return i.name == "" },
			wantFound: true,
			want:      item{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Find(tt.slice, tt.predicate)

			if !tt.wantFound {
				if got != nil {
					t.Errorf("Find() = %v, want nil", *got)
				}

				return
			}

			if got == nil {
				t.Errorf("Find() = nil, want %v", tt.want)

				return
			}

			if *got != tt.want {
				t.Errorf("Find() = %v, want %v", *got, tt.want)
			}
		})
	}
}
