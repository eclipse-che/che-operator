//
// Copyright (c) 2019-2021 Red Hat, Inc.
// This program and the accompanying materials are made
// available under the terms of the Eclipse Public License 2.0
// which is available at https://www.eclipse.org/legal/epl-2.0/
//
// SPDX-License-Identifier: EPL-2.0
//
// Contributors:
//   Red Hat, Inc. - initial API and implementation
//
package defaults

import (
	"fmt"
	"testing"

	chev2 "github.com/eclipse-che/che-operator/api/v2"
)

func TestCorrectImageName(t *testing.T) {
	testCases := map[string]string{
		"docker.io/eclipse/che-operator:latest": "che-operator:latest",
		"eclipse/che-operator:7.1.0":            "che-operator:7.1.0",
		"che-operator:7.2.0":                    "che-operator:7.2.0",
	}
	for k, v := range testCases {
		t.Run(k, func(*testing.T) {
			actual := getImageNameFromFullImage(k)
			if actual != v {
				t.Errorf("Expected %s but was %s", v, actual)
			}
		})
	}
}

func TestCorrectAirGapPatchedImage(t *testing.T) {
	type testcase struct {
		image    string
		expected string
		cr       *chev2.CheCluster
	}

	var (
		airGapRegistryHostname                            = "myregistry.org"
		airGapRegistryOrganization                        = "myorg"
		expectedAirGapPostgresUpstreamImage               = makeAirGapImagePath(airGapRegistryHostname, airGapRegistryOrganization, getImageNameFromFullImage(defaultPostgresImage))
		expectedAirGapPostgresUpstreamImageOnlyOrgChanged = makeAirGapImagePath(getHostnameFromImage(defaultPostgresImage), airGapRegistryOrganization, getImageNameFromFullImage(defaultPostgresImage))
	)

	upstream := &chev2.CheCluster{}
	airGapUpstream := &chev2.CheCluster{
		Spec: chev2.CheClusterSpec{
			ContainerRegistry: chev2.CheClusterContainerRegistry{
				Hostname:     airGapRegistryHostname,
				Organization: airGapRegistryOrganization,
			},
		},
	}
	upstreamOnlyOrg := &chev2.CheCluster{
		Spec: chev2.CheClusterSpec{
			ContainerRegistry: chev2.CheClusterContainerRegistry{
				Organization: airGapRegistryOrganization,
			},
		},
	}

	testCases := map[string]testcase{
		"default postgres":          {image: defaultPostgresImage, expected: defaultPostgresImage, cr: upstream},
		"airgap postgres":           {image: defaultPostgresImage, expected: expectedAirGapPostgresUpstreamImage, cr: airGapUpstream},
		"with only the org changed": {image: defaultPostgresImage, expected: expectedAirGapPostgresUpstreamImageOnlyOrgChanged, cr: upstreamOnlyOrg},
	}
	for name, tc := range testCases {
		t.Run(name, func(*testing.T) {
			actual := PatchDefaultImageName(tc.cr, tc.image)
			if actual != tc.expected {
				t.Errorf("Expected %s but was %s", tc.expected, actual)
			}
		})
	}
}

func makeAirGapImagePath(hostname, org, nameAndTag string) string {
	return fmt.Sprintf("%s/%s/%s", hostname, org, nameAndTag)
}
