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
	"io/ioutil"
	"os"
	"testing"

	orgv1 "github.com/eclipse/che-operator/pkg/apis/org/v1"
	"gopkg.in/yaml.v2"
	appsv1 "k8s.io/api/apps/v1"
)

var (
	cheVersionTest           string
	cheServerImageTest       string
	pluginRegistryImageTest  string
	devfileRegistryImageTest string
	pvcJobsImageTest         string
	postgresImageTest        string
	keycloakImageTest        string
	brokerMetadataTest       string
	brokerArtifactsTest      string
	jwtProxyTest             string
	tlsJobImageTest          string
)

func init() {
	operator := &appsv1.Deployment{}
	data, err := ioutil.ReadFile("../../deploy/operator.yaml")
	yaml.Unmarshal(data, operator)
	if err == nil {
		for _, env := range operator.Spec.Template.Spec.Containers[0].Env {
			os.Setenv(env.Name, env.Value)
			switch env.Name {
			case "CHE_VERSION":
				cheVersionTest = env.Value
			case "RELATED_IMAGE_che_server":
				cheServerImageTest = env.Value
			case "RELATED_IMAGE_plugin_registry":
				pluginRegistryImageTest = env.Value
			case "RELATED_IMAGE_devfile_registry":
				devfileRegistryImageTest = env.Value
			case "RELATED_IMAGE_che_tls_secrets_creation_job":
				tlsJobImageTest = env.Value
			case "RELATED_IMAGE_pvc_jobs":
				pvcJobsImageTest = env.Value
			case "RELATED_IMAGE_postgres":
				postgresImageTest = env.Value
			case "RELATED_IMAGE_keycloak":
				keycloakImageTest = env.Value
			case "RELATED_IMAGE_che_workspace_plugin_broker_metadata":
				brokerMetadataTest = env.Value
			case "RELATED_IMAGE_che_workspace_plugin_broker_artifacts":
				brokerArtifactsTest = env.Value
			case "RELATED_IMAGE_che_server_secure_exposer_jwt_proxy_image":
				jwtProxyTest = env.Value
			}
		}
	}

	InitDefaultsFromEnv()
}

func TestDefaultFromEnv(t *testing.T) {
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
