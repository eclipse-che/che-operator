package deploy

import (
	"fmt"
	"testing"

	orgv1 "github.com/eclipse/che-operator/pkg/apis/org/v1"
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
		cr       *orgv1.CheCluster
	}

	var (
		airGapRegistryHostname                                   = "myregistry.org"
		airGapRegistryOrganization                               = "myorg"
		expectedAirGapPostgresUpstreamImage                      = makeAirGapImagePath(airGapRegistryHostname, airGapRegistryOrganization, getImageNameFromFullImage(defaultPostgresUpstreamImage))
		expectedAirGapPostgresUpstreamImageOnlyOrgChanged        = makeAirGapImagePath(getHostnameFromImage(defaultPostgresUpstreamImage), airGapRegistryOrganization, getImageNameFromFullImage(defaultPostgresUpstreamImage))
		expectedAirGapCRWPluginRegistryOnlyOrgChanged            = makeAirGapImagePath(getHostnameFromImage(defaultPluginRegistryImage), airGapRegistryOrganization, getImageNameFromFullImage(defaultPluginRegistryImage))
		expectedAirGapCRWPostgresImage                           = makeAirGapImagePath(airGapRegistryHostname, airGapRegistryOrganization, getImageNameFromFullImage(defaultPostgresImage))
		expectedAirGapKeyCloakImageOnlyHostnameChanged           = makeAirGapImagePath(airGapRegistryHostname, getOrganizationFromImage(defaultKeycloakUpstreamImage), getImageNameFromFullImage(defaultKeycloakUpstreamImage))
		expectedAirGapCRWDevfileRegistryImageOnlyHostnameChanged = makeAirGapImagePath(airGapRegistryHostname, getOrganizationFromImage(defaultDevfileRegistryImage), getImageNameFromFullImage(defaultDevfileRegistryImage))
	)

	upstream := &orgv1.CheCluster{
		Spec: orgv1.CheClusterSpec{
			Server: orgv1.CheClusterSpecServer{},
		},
	}
	crw := &orgv1.CheCluster{
		Spec: orgv1.CheClusterSpec{
			Server: orgv1.CheClusterSpecServer{
				CheFlavor: "codeready",
			},
		},
	}
	airGapUpstream := &orgv1.CheCluster{
		Spec: orgv1.CheClusterSpec{
			Server: orgv1.CheClusterSpecServer{
				AirGapContainerRegistryHostname:     airGapRegistryHostname,
				AirGapContainerRegistryOrganization: airGapRegistryOrganization,
			},
		},
	}
	airGapCRW := &orgv1.CheCluster{
		Spec: orgv1.CheClusterSpec{
			Server: orgv1.CheClusterSpecServer{
				AirGapContainerRegistryHostname:     airGapRegistryHostname,
				AirGapContainerRegistryOrganization: airGapRegistryOrganization,
				CheFlavor:                           "codeready",
			},
		},
	}
	upstreamOnlyOrg := &orgv1.CheCluster{
		Spec: orgv1.CheClusterSpec{
			Server: orgv1.CheClusterSpecServer{
				AirGapContainerRegistryOrganization: airGapRegistryOrganization,
			},
		},
	}
	upstreamOnlyHostname := &orgv1.CheCluster{
		Spec: orgv1.CheClusterSpec{
			Server: orgv1.CheClusterSpecServer{
				AirGapContainerRegistryHostname: airGapRegistryHostname,
			},
		},
	}
	crwOnlyOrg := &orgv1.CheCluster{
		Spec: orgv1.CheClusterSpec{
			Server: orgv1.CheClusterSpecServer{
				AirGapContainerRegistryOrganization: airGapRegistryOrganization,
				CheFlavor:                           "codeready",
			},
		},
	}
	crwOnlyHostname := &orgv1.CheCluster{
		Spec: orgv1.CheClusterSpec{
			Server: orgv1.CheClusterSpecServer{
				AirGapContainerRegistryHostname: airGapRegistryHostname,
				CheFlavor:                       "codeready",
			},
		},
	}

	testCases := map[string]testcase{
		"upstream default postgres":                           {image: defaultPostgresUpstreamImage, expected: defaultPostgresUpstreamImage, cr: upstream},
		"airgap upstream postgres":                            {image: defaultPostgresUpstreamImage, expected: expectedAirGapPostgresUpstreamImage, cr: airGapUpstream},
		"upstream with only the org changed":                  {image: defaultPostgresUpstreamImage, expected: expectedAirGapPostgresUpstreamImageOnlyOrgChanged, cr: upstreamOnlyOrg},
		"codeready plugin registry with only the org changed": {image: defaultPluginRegistryImage, expected: expectedAirGapCRWPluginRegistryOnlyOrgChanged, cr: crwOnlyOrg},
		"CRW postgres":                                        {image: defaultPostgresImage, expected: defaultPostgresImage, cr: crw},
		"CRW airgap postgres":                                 {image: defaultPostgresImage, expected: expectedAirGapCRWPostgresImage, cr: airGapCRW},
		"upstream airgap with only hostname defined":          {image: defaultKeycloakUpstreamImage, expected: expectedAirGapKeyCloakImageOnlyHostnameChanged, cr: upstreamOnlyHostname},
		"crw airgap with only hostname defined":               {image: defaultDevfileRegistryImage, expected: expectedAirGapCRWDevfileRegistryImageOnlyHostnameChanged, cr: crwOnlyHostname},
	}
	for name, tc := range testCases {
		t.Run(name, func(*testing.T) {
			actual := patchDefaultImageName(tc.cr, tc.image)
			if actual != tc.expected {
				t.Errorf("Expected %s but was %s", tc.expected, actual)
			}
		})
	}
}

func makeAirGapImagePath(hostname, org, nameAndTag string) string {
	return fmt.Sprintf("%s/%s/%s", hostname, org, nameAndTag)
}
