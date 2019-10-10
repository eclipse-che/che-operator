package deploy

import (
	orgv1 "github.com/eclipse/che-operator/pkg/apis/org/v1"
	"testing"
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
				AirGapContainerRegistryHostname:     "bigcorp.net",
				AirGapContainerRegistryOrganization: "che-images",
			},
		},
	}
	airGapCRW := &orgv1.CheCluster{
		Spec: orgv1.CheClusterSpec{
			Server: orgv1.CheClusterSpecServer{
				AirGapContainerRegistryHostname:     "bigcorp.net",
				AirGapContainerRegistryOrganization: "che-images",
				CheFlavor:                           "codeready",
			},
		},
	}
	upstreamOnlyOrg := &orgv1.CheCluster{
		Spec: orgv1.CheClusterSpec{
			Server: orgv1.CheClusterSpecServer{
				AirGapContainerRegistryOrganization: "che-images",
			},
		},
	}
	upstreamOnlyHostname := &orgv1.CheCluster{
		Spec: orgv1.CheClusterSpec{
			Server: orgv1.CheClusterSpecServer{
				AirGapContainerRegistryHostname: "bigcorp.net",
			},
		},
	}
	crwOnlyOrg := &orgv1.CheCluster{
		Spec: orgv1.CheClusterSpec{
			Server: orgv1.CheClusterSpecServer{
				AirGapContainerRegistryOrganization: "che-images",
				CheFlavor:                           "codeready",
			},
		},
	}
	crwOnlyHostname := &orgv1.CheCluster{
		Spec: orgv1.CheClusterSpec{
			Server: orgv1.CheClusterSpecServer{
				AirGapContainerRegistryHostname: "bigcorp.net",
				CheFlavor:                       "codeready",
			},
		},
	}

	testCases := map[string]testcase{
		"upstream default postgres":                           {image: defaultPostgresUpstreamImage, expected: defaultPostgresUpstreamImage, cr: upstream},
		"airgap upstream postgres":                            {image: defaultPostgresUpstreamImage, expected: "bigcorp.net/che-images/postgresql-96-centos7:9.6", cr: airGapUpstream},
		"upstream with only the org changed":                  {image: defaultPostgresUpstreamImage, expected: "docker.io/che-images/postgresql-96-centos7:9.6", cr: upstreamOnlyOrg},
		"codeready plugin registry with only the org changed": {image: defaultPluginRegistryImage, expected: "registry.redhat.io/che-images/pluginregistry-rhel8:2.0", cr: crwOnlyOrg},
		"CRW postgres":                                        {image: defaultPostgresImage, expected: defaultPostgresImage, cr: crw},
		"CRW airgap postgres":                                 {image: defaultPostgresImage, expected: "bigcorp.net/che-images/postgresql-96-rhel7:1-47", cr: airGapCRW},
		"upstream airgap with only hostname defined":          {image: defaultKeycloakUpstreamImage, expected: "bigcorp.net/eclipse/che-keycloak:7.2.0", cr: upstreamOnlyHostname},
		"crw airgap with only hostname defined":               {image: defaultDevfileRegistryImage, expected: "bigcorp.net/codeready-workspaces/devfileregistry-rhel8:2.0", cr: crwOnlyHostname},
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
