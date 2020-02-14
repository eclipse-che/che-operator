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

	orgv1 "github.com/eclipse/che-operator/pkg/apis/org/v1"
)

const (
	cheServerImageTag    = "7.8.0"
	cheServerImage       = "quay.io/eclipse/che-server"
	pluginRegistryImage  = "quay.io/eclipse/che-plugin-registry:7.8.0"
	devfileRegistryImage = "quay.io/eclipse/che-devfile-registry:7.8.0"
	pvcJobsImage         = "registry.access.redhat.com/ubi8-minimal:8.0-213"
	postgresImage        = "centos/postgresql-96-centos7:9.6"
	keycloakImage        = "quay.io/eclipse/che-keycloak:7.8.0"
)

func init() {
	os.Setenv("DEFAULT_CHE_SERVER_IMAGE_TAG", cheServerImageTag)
	os.Setenv("DEFAULT_CHE_SERVER_IMAGE_REPO", cheServerImage)
	os.Setenv("IMAGE_default_plugin_registry", pluginRegistryImage)
	os.Setenv("IMAGE_default_devfile_registry", devfileRegistryImage)
	os.Setenv("IMAGE_default_pvc_jobs", pvcJobsImage)
	os.Setenv("IMAGE_default_postgres", postgresImage)
	os.Setenv("IMAGE_default_keycloak", keycloakImage)
	os.Setenv("IMAGE_default_che_workspace_plugin_broker_metadata", "quay.io/crw/pluginbroker-metadata-rhel8:2.1")
	os.Setenv("IMAGE_default_che_workspace_plugin_broker_artifacts", "quay.io/crw/pluginbroker-artifacts-rhel8:2.1")
	os.Setenv("IMAGE_default_che_server_secure_exposer_jwt_proxy_image", "quay.io/crw/jwtproxy-rhel8:2.1")

	InitDefaultsFromEnv()
}

func TestDefaultFromEnv(t *testing.T) {
	if DefaultCheServerImageTag() != cheServerImageTag {
		t.Errorf("Expected %s but was %s", cheServerImageTag, DefaultCheServerImageTag())
	}

	cheCluster := &orgv1.CheCluster{
		Spec: orgv1.CheClusterSpec{
			Server: orgv1.CheClusterSpecServer{},
		},
	}
	cheFlavor := "che"

	if DefaultCheServerImageRepo(cheCluster) != cheServerImage {
		t.Errorf("Expected %s but was %s", cheServerImage, DefaultCheServerImageRepo(cheCluster))
	}

	if DefaultPluginRegistryImage(cheCluster) != pluginRegistryImage {
		t.Errorf("Expected %s but was %s", pluginRegistryImage, DefaultPluginRegistryImage(cheCluster))
	}

	if DefaultDevfileRegistryImage(cheCluster) != devfileRegistryImage {
		t.Errorf("Expected %s but was %s", devfileRegistryImage, DefaultDevfileRegistryImage(cheCluster))
	}

	if DefaultPvcJobsImage(cheCluster) != pvcJobsImage {
		t.Errorf("Expected %s but was %s", pvcJobsImage, DefaultPvcJobsImage(cheCluster))
	}

	if DefaultPostgresImage(cheCluster) != postgresImage {
		t.Errorf("Expected %s but was %s", postgresImage, DefaultPostgresImage(cheCluster))
	}

	if DefaultKeycloakImage(cheCluster) != keycloakImage {
		t.Errorf("Expected %s but was %s", keycloakImage, DefaultKeycloakImage(cheCluster))
	}

	if DefaultCheWorkspacePluginBrokerMetadataImage(cheCluster, cheFlavor) != "" {
		t.Errorf("Expected empty value for cheFlavor '%s', but was %s", cheFlavor, DefaultCheWorkspacePluginBrokerMetadataImage(cheCluster, cheFlavor))
	}

	if DefaultCheWorkspacePluginBrokerArtifactsImage(cheCluster, cheFlavor) != "" {
		t.Errorf("Expected empty value for cheFlavor '%s', but was %s", cheFlavor, DefaultCheWorkspacePluginBrokerArtifactsImage(cheCluster, cheFlavor))
	}

	if DefaultCheServerSecureExposerJwtProxyImage(cheCluster, cheFlavor) != "" {
		t.Errorf("Expected empty value for cheFlavor '%s', but was %s", cheFlavor, DefaultCheWorkspacePluginBrokerArtifactsImage(cheCluster, cheFlavor))
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
