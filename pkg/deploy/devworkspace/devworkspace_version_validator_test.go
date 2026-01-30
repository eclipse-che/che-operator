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

package devworkspace

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExtractVersionFromCSV(t *testing.T) {
	tests := []struct {
		name           string
		csvName        string
		wantVersion    string
		wantDevVersion string
	}{
		{
			name:           "Valid CSV with dev version",
			csvName:        "devworkspace-operator.v0.40.0-dev.3",
			wantVersion:    "0.40.0",
			wantDevVersion: "dev.3",
		},
		{
			name:           "Valid CSV without dev version",
			csvName:        "devworkspace-operator.v0.40.0",
			wantVersion:    "0.40.0",
			wantDevVersion: "",
		},
		{
			name:           "Valid CSV with patch version",
			csvName:        "devworkspace-operator.v1.2.3",
			wantVersion:    "1.2.3",
			wantDevVersion: "",
		},
		{
			name:           "Valid CSV with complex dev version",
			csvName:        "devworkspace-operator.v0.40.0-alpha.1",
			wantVersion:    "0.40.0",
			wantDevVersion: "alpha.1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotVersion, gotDevVersion, err := extractVersionFromCSV(tt.csvName)
			assert.NoError(t, err)
			assert.Equal(t, tt.wantVersion, gotVersion)
			assert.Equal(t, tt.wantDevVersion, gotDevVersion)
		})
	}
}
