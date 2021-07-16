//
// Copyright (c) 2012-2019 Red Hat, Inc.
// This program and the accompanying materials are made
// available under the terms of the Eclipse Public License 2.0
// which is available at https://www.eclipse.org/legal/epl-2.0/
//
// SPDX-License-Identifier: EPL-2.0
//
// Contributors:
//   Red Hat, Inc. - initial API and implementation
//
package deploy

import (
	"fmt"
	"os"
	"testing"

	"github.com/eclipse-che/che-operator/pkg/util"

	orgv1 "github.com/eclipse-che/che-operator/api/v1"
)

func TestDefaultFromEnv(t *testing.T) {

	cheVersionTest := os.Getenv("CHE_VERSION")

	cheServerImageTest := os.Getenv(util.GetArchitectureDependentEnv("RELATED_IMAGE_che_server"))
	dashboardImageTest := os.Getenv(util.GetArchitectureDependentEnv("RELATED_IMAGE_dashboard"))
	pluginRegistryImageTest := os.Getenv(util.GetArchitectureDependentEnv("RELATED_IMAGE_plugin_registry"))
	devfileRegistryImageTest := os.Getenv(util.GetArchitectureDependentEnv("RELATED_IMAGE_devfile_registry"))
	pvcJobsImageTest := os.Getenv(util.GetArchitectureDependentEnv("RELATED_IMAGE_pvc_jobs"))
	postgresImageTest := os.Getenv(util.GetArchitectureDependentEnv("RELATED_IMAGE_postgres"))
	keycloakImageTest := os.Getenv(util.GetArchitectureDependentEnv("RELATED_IMAGE_keycloak"))
	brokerMetadataTest := os.Getenv(util.GetArchitectureDependentEnv("RELATED_IMAGE_che_workspace_plugin_broker_metadata"))
	brokerArtifactsTest := os.Getenv(util.GetArchitectureDependentEnv("RELATED_IMAGE_che_workspace_plugin_broker_artifacts"))
	jwtProxyTest := os.Getenv(util.GetArchitectureDependentEnv("RELATED_IMAGE_che_server_secure_exposer_jwt_proxy_image"))

	if DefaultCheVersion() != cheVersionTest {
		t.Errorf("Expected %s but was %s", cheVersionTest, DefaultCheVersion())
	}

	cheCluster := &orgv1.CheCluster{
		Spec: orgv1.CheClusterSpec{
			Server: orgv1.CheClusterSpecServer{},
		},
	}

	if DefaultCheServerImage(cheCluster) != cheServerImageTest {
		t.Errorf("Expected %s but was %s", cheServerImageTest, DefaultCheServerImage(cheCluster))
	}

	if DefaultDashboardImage(cheCluster) != dashboardImageTest {
		t.Errorf("Expected %s but was %s", dashboardImageTest, DefaultDashboardImage(cheCluster))
	}

	if DefaultPluginRegistryImage(cheCluster) != pluginRegistryImageTest {
		t.Errorf("Expected %s but was %s", pluginRegistryImageTest, DefaultPluginRegistryImage(cheCluster))
	}

	if DefaultDevfileRegistryImage(cheCluster) != devfileRegistryImageTest {
		t.Errorf("Expected %s but was %s", devfileRegistryImageTest, DefaultDevfileRegistryImage(cheCluster))
	}

	if DefaultPvcJobsImage(cheCluster) != pvcJobsImageTest {
		t.Errorf("Expected %s but was %s", pvcJobsImageTest, DefaultPvcJobsImage(cheCluster))
	}

	if DefaultPostgresImage(cheCluster) != postgresImageTest {
		t.Errorf("Expected %s but was %s", postgresImageTest, DefaultPostgresImage(cheCluster))
	}

	if DefaultKeycloakImage(cheCluster) != keycloakImageTest {
		t.Errorf("Expected %s but was %s", keycloakImageTest, DefaultKeycloakImage(cheCluster))
	}

	if DefaultCheWorkspacePluginBrokerMetadataImage(cheCluster) != brokerMetadataTest {
		t.Errorf("Expected '%s', but was %s", brokerMetadataTest, DefaultCheWorkspacePluginBrokerMetadataImage(cheCluster))
	}

	if DefaultCheWorkspacePluginBrokerArtifactsImage(cheCluster) != brokerArtifactsTest {
		t.Errorf("Expected '%s', but was %s", brokerArtifactsTest, DefaultCheWorkspacePluginBrokerArtifactsImage(cheCluster))
	}

	if DefaultCheServerSecureExposerJwtProxyImage(cheCluster) != jwtProxyTest {
		t.Errorf("Expected '%s', but was %s", jwtProxyTest, DefaultCheWorkspacePluginBrokerArtifactsImage(cheCluster))
	}
}

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
		expectedAirGapPostgresUpstreamImage                      = makeAirGapImagePath(airGapRegistryHostname, airGapRegistryOrganization, getImageNameFromFullImage(defaultPostgresImage))
		expectedAirGapPostgresUpstreamImageOnlyOrgChanged        = makeAirGapImagePath(getHostnameFromImage(defaultPostgresImage), airGapRegistryOrganization, getImageNameFromFullImage(defaultPostgresImage))
		expectedAirGapCRWPluginRegistryOnlyOrgChanged            = makeAirGapImagePath(getHostnameFromImage(defaultPluginRegistryImage), airGapRegistryOrganization, getImageNameFromFullImage(defaultPluginRegistryImage))
		expectedAirGapCRWPostgresImage                           = makeAirGapImagePath(airGapRegistryHostname, airGapRegistryOrganization, getImageNameFromFullImage(defaultPostgresImage))
		expectedAirGapKeyCloakImageOnlyHostnameChanged           = makeAirGapImagePath(airGapRegistryHostname, getOrganizationFromImage(defaultKeycloakImage), getImageNameFromFullImage(defaultKeycloakImage))
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
		"default postgres":          {image: defaultPostgresImage, expected: defaultPostgresImage, cr: upstream},
		"airgap postgres":           {image: defaultPostgresImage, expected: expectedAirGapPostgresUpstreamImage, cr: airGapUpstream},
		"with only the org changed": {image: defaultPostgresImage, expected: expectedAirGapPostgresUpstreamImageOnlyOrgChanged, cr: upstreamOnlyOrg},
		"codeready plugin registry with only the org changed": {image: defaultPluginRegistryImage, expected: expectedAirGapCRWPluginRegistryOnlyOrgChanged, cr: crwOnlyOrg},
		"CRW postgres":                          {image: defaultPostgresImage, expected: defaultPostgresImage, cr: crw},
		"CRW airgap postgres":                   {image: defaultPostgresImage, expected: expectedAirGapCRWPostgresImage, cr: airGapCRW},
		"airgap with only hostname defined":     {image: defaultKeycloakImage, expected: expectedAirGapKeyCloakImageOnlyHostnameChanged, cr: upstreamOnlyHostname},
		"crw airgap with only hostname defined": {image: defaultDevfileRegistryImage, expected: expectedAirGapCRWDevfileRegistryImageOnlyHostnameChanged, cr: crwOnlyHostname},
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
