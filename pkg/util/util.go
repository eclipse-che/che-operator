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
package util

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	orgv1 "github.com/eclipse-che/che-operator/api/v1"
	routev1 "github.com/openshift/api/route/v1"
	"github.com/sirupsen/logrus"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/discovery"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/yaml"
)

var (
	k8sclient                    = GetK8Client()
	IsOpenShift, IsOpenShift4, _ = DetectOpenShift()
)

func ContainsString(slice []string, s string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
}

func DoRemoveString(slice []string, s string) (result []string) {
	for _, item := range slice {
		if item == s {
			continue
		}
		result = append(result, item)
	}
	return
}

func GeneratePasswd(stringLength int) (passwd string) {
	rand.Seed(time.Now().UnixNano())
	chars := []rune("ABCDEFGHIJKLMNOPQRSTUVWXYZ" +
		"abcdefghijklmnopqrstuvwxyz" +
		"0123456789")
	length := stringLength
	buf := make([]rune, length)
	for i := range buf {
		buf[i] = chars[rand.Intn(len(chars))]
	}
	passwd = string(buf)
	return passwd
}

func MapToKeyValuePairs(m map[string]string) string {
	buff := new(bytes.Buffer)
	keys := make([]string, 0, len(m))

	for key := range m {
		keys = append(keys, key)
	}

	sort.Strings(keys) //sort keys alphabetically

	for _, key := range keys {
		fmt.Fprintf(buff, "%s=%s,", key, m[key])
	}
	return strings.TrimSuffix(buff.String(), ",")
}

func DetectOpenShift() (isOpenshift bool, isOpenshift4 bool, anError error) {
	tests := IsTestMode()
	if tests {
		openshiftVersionEnv := os.Getenv("OPENSHIFT_VERSION")
		openshiftVersion, err := strconv.ParseInt(openshiftVersionEnv, 0, 64)
		if err == nil && openshiftVersion == 4 {
			return true, true, nil
		}
		return true, false, nil
	}

	apiGroups, err := getApiList()
	if err != nil {
		return false, false, err
	}
	for _, apiGroup := range apiGroups {
		if apiGroup.Name == "route.openshift.io" {
			isOpenshift = true
		}
		if apiGroup.Name == "config.openshift.io" {
			isOpenshift4 = true
		}
	}

	return isOpenshift, isOpenshift4, nil
}

func getDiscoveryClient() (*discovery.DiscoveryClient, error) {
	kubeconfig, err := config.GetConfig()
	if err != nil {
		return nil, err
	}
	return discovery.NewDiscoveryClientForConfig(kubeconfig)
}

func getApiList() ([]metav1.APIGroup, error) {
	discoveryClient, err := getDiscoveryClient()
	if err != nil {
		return nil, err
	}
	apiList, err := discoveryClient.ServerGroups()
	if err != nil {
		return nil, err
	}
	return apiList.Groups, nil
}

func HasK8SResourceObject(discoveryClient discovery.DiscoveryInterface, resourceName string) bool {
	_, resourceList, err := discoveryClient.ServerGroupsAndResources()
	if err != nil {
		return false
	}

	return HasAPIResourceNameInList(resourceName, resourceList)
}

func HasAPIResourceNameInList(name string, resources []*metav1.APIResourceList) bool {
	for _, l := range resources {
		for _, r := range l.APIResources {
			if r.Name == name {
				return true
			}
		}
	}

	return false
}

func GetValue(key string, defaultValue string) (value string) {
	value = key
	if len(key) < 1 {
		value = defaultValue
	}
	return value
}

func GetMapValue(value map[string]string, defaultValue map[string]string) map[string]string {
	ret := value
	if len(value) < 1 {
		ret = defaultValue
	}

	return ret
}

func MergeMaps(first map[string]string, second map[string]string) map[string]string {
	ret := make(map[string]string)
	for k, v := range first {
		ret[k] = v
	}

	for k, v := range second {
		ret[k] = v
	}

	return ret
}

func IsTestMode() (isTesting bool) {
	testMode := os.Getenv("MOCK_API")
	if len(testMode) == 0 {
		return false
	}
	return true
}

func GetRouterCanonicalHostname(client client.Client, namespace string) (string, error) {
	testRouteYaml, err := GetTestRouteYaml(client, namespace)
	if err != nil {
		return "", err
	}
	return testRouteYaml.Status.Ingress[0].RouterCanonicalHostname, nil
}

// GetTestRouteYaml creates test route and returns its spec.
func GetTestRouteYaml(client client.Client, namespace string) (*routev1.Route, error) {
	// Create test route to get the info
	routeSpec := &routev1.Route{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Route",
			APIVersion: routev1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "canonical-hostname-route-name",
			Namespace: namespace,
		},
		Spec: routev1.RouteSpec{
			To: routev1.RouteTargetReference{
				Kind: "Service",
				Name: "canonical-hostname-route-nonexisting-service",
			},
			Port: &routev1.RoutePort{
				TargetPort: intstr.IntOrString{
					Type:   intstr.Int,
					IntVal: 8080,
				},
			},
		},
	}

	if err := client.Create(context.TODO(), routeSpec); err != nil {
		if !k8sErrors.IsAlreadyExists(err) {
			return nil, err
		}
	}

	// Schedule test route cleanup after the job done.
	defer func() {
		if err := client.Delete(context.TODO(), routeSpec); err != nil {
			logrus.Errorf("Failed to delete test route %s: %s", routeSpec.Name, err)
		}
	}()

	// Wait till the route is ready
	route := &routev1.Route{}
	errCount := 0
	for {
		time.Sleep(time.Duration(1) * time.Second)

		routeNsName := types.NamespacedName{Name: routeSpec.Name, Namespace: namespace}
		err := client.Get(context.TODO(), routeNsName, route)
		if err == nil {
			return route, nil
		} else if !k8sErrors.IsNotFound(err) {
			errCount++
			if errCount > 10 {
				return nil, err
			}
		} else {
			// Reset counter as got not found error
			errCount = 0
		}
	}
}

func GetDeploymentEnv(deployment *appsv1.Deployment, key string) (value string) {
	env := deployment.Spec.Template.Spec.Containers[0].Env
	for i := range env {
		name := env[i].Name
		if name == key {
			value = env[i].Value
			break
		}
	}
	return value
}

func GetDeploymentEnvVarSource(deployment *appsv1.Deployment, key string) (valueFrom *corev1.EnvVarSource) {
	env := deployment.Spec.Template.Spec.Containers[0].Env
	for i := range env {
		name := env[i].Name
		if name == key {
			valueFrom = env[i].ValueFrom
			break
		}
	}
	return valueFrom
}

func GetEnvByRegExp(regExp string) []corev1.EnvVar {
	var env []corev1.EnvVar
	for _, e := range os.Environ() {
		pair := strings.SplitN(e, "=", 2)
		envName := pair[0]
		rxp := regexp.MustCompile(regExp)
		if rxp.MatchString(envName) {
			envName = GetArchitectureDependentEnv(envName)
			env = append(env, corev1.EnvVar{Name: envName, Value: pair[1]})
		}
	}
	return env
}

// GetArchitectureDependentEnv returns environment variable dependending on architecture
// by adding "_<ARCHITECTURE>" suffix. If variable is not set then the default will be return.
func GetArchitectureDependentEnv(env string) string {
	archEnv := env + "_" + runtime.GOARCH
	if _, ok := os.LookupEnv(archEnv); ok {
		return archEnv
	}

	return env
}

// GetImageNameAndTag returns the image repository and tag name from the provided image
//
// Referenced from https://github.com/che-incubator/chectl/blob/main/src/util.ts
func GetImageNameAndTag(image string) (string, string) {
	var imageName, imageTag string
	if strings.Contains(image, "@") {
		// Image is referenced via a digest
		index := strings.Index(image, "@")
		imageName = image[:index]
		imageTag = image[index+1:]
	} else {
		// Image is referenced via a tag
		lastColonIndex := strings.LastIndex(image, ":")
		if lastColonIndex == -1 {
			imageName = image
			imageTag = "latest"
		} else {
			beforeLastColon := image[:lastColonIndex]
			afterLastColon := image[lastColonIndex+1:]
			if strings.Contains(afterLastColon, "/") {
				// The colon is for registry port and not for a tag
				imageName = image
				imageTag = "latest"
			} else {
				// The colon separates image name from the tag
				imageName = beforeLastColon
				imageTag = afterLastColon
			}
		}
	}
	return imageName, imageTag
}

// NewBoolPointer returns `bool` pointer to value in the memory.
// Unfortunately golang hasn't got syntax to create `bool` pointer.
func NewBoolPointer(value bool) *bool {
	variable := value
	return &variable
}

func GetResourceQuantity(value string, defaultValue string) resource.Quantity {
	if value != "" {
		return resource.MustParse(value)
	}
	return resource.MustParse(defaultValue)
}

// Finds Env by a given name
func FindEnv(envs []corev1.EnvVar, name string) *corev1.EnvVar {
	for _, env := range envs {
		if env.Name == name {
			return &env
		}
	}

	return nil
}

func ReadObject(yamlFile string, obj interface{}) error {
	data, err := ioutil.ReadFile(yamlFile)
	if err != nil {
		return err
	}

	err = yaml.Unmarshal(data, obj)
	if err != nil {
		return err
	}

	return nil
}

func ComputeHash256(data []byte) string {
	hasher := sha256.New()
	hasher.Write(data)
	return base64.URLEncoding.EncodeToString(hasher.Sum(nil))
}

func UpdateBackupServerConfiguration(client client.Client, backupServerConfig *orgv1.CheBackupServerConfiguration) error {
	err := client.Update(context.TODO(), backupServerConfig)
	if err != nil {
		logrus.Errorf("Failed to update %s CR: %s", backupServerConfig.Name, err.Error())
		return err
	}
	return nil
}

func UpdateBackupServerConfigurationStatus(client client.Client, backupServerConfig *orgv1.CheBackupServerConfiguration) error {
	err := client.Status().Update(context.TODO(), backupServerConfig)
	if err != nil {
		logrus.Errorf("Failed to update %s CR status: %s", backupServerConfig.Name, err.Error())
		return err
	}
	return nil
}

func IsCheMultiUser(cheCluster *orgv1.CheCluster) bool {
	return cheCluster.Spec.Server.CustomCheProperties == nil || cheCluster.Spec.Server.CustomCheProperties["CHE_MULTIUSER"] != "false"
}

// ClearMetadata removes extra fields from given metadata.
// It is required to remove ResourceVersion in order to be able to apply the yaml again.
func ClearMetadata(objectMeta *metav1.ObjectMeta) {
	objectMeta.ResourceVersion = ""
	objectMeta.Finalizers = []string{}
	objectMeta.ManagedFields = []metav1.ManagedFieldsEntry{}
}

// GetCheURL returns Che url.
func GetCheURL(cheCluster *orgv1.CheCluster) string {
	return "https://" + cheCluster.Spec.Server.CheHost
}
