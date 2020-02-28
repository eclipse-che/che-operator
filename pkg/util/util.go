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
package util

import (
	"sort"
	"crypto/tls"
	"encoding/json"
	"errors"
	"github.com/sirupsen/logrus"
	"io/ioutil"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/discovery"
	"math/rand"
	"net/http"
	"os"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"strings"
	"time"
	"bytes"
	"fmt"
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
	if !tests {
		apiGroups, err := getApiList()
		if err != nil{
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
		return
	}
	return true, false, nil
}

func getDiscoveryClient() (*discovery.DiscoveryClient, error) {
	kubeconfig, err := config.GetConfig()
	if err != nil {
		return nil, err
	}
	return discovery.NewDiscoveryClientForConfig(kubeconfig)
}

func getApiList() ([]v1.APIGroup, error) {
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

func GetServerResources() ([]*v1.APIResourceList, error) {
	discoveryClient, err := getDiscoveryClient()
	if err != nil {
		return nil, err
	}
	return discoveryClient.ServerResources()
}

func GetValue(key string, defaultValue string) (value string) {

	value = key
	if len(key) < 1 {
		value = defaultValue
	}
	return value
}

func IsTestMode() (isTesting bool) {

	testMode := os.Getenv("MOCK_API")
	if len(testMode) == 0 {
		return false
	}
	return true
}

func GetClusterPublicHostname(isOpenShift4 bool) (hostname string, err error) {
	if isOpenShift4 {
		return getClusterPublicHostnameForOpenshiftV4()
	} else {
		return getClusterPublicHostnameForOpenshiftV3()
	}
}

// getClusterPublicHostnameForOpenshiftV3 is a hacky way to get OpenShift API public DNS/IP
// to be used in OpenShift oAuth provider as baseURL
func getClusterPublicHostnameForOpenshiftV3() (hostname string, err error) {
	http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	client := &http.Client{}
	kubeApi := os.Getenv("KUBERNETES_PORT_443_TCP_ADDR")
	url := "https://" + kubeApi + "/.well-known/oauth-authorization-server"
	req, err := http.NewRequest(http.MethodGet, url, nil)
	resp, err := client.Do(req)
	if err != nil {
		logrus.Errorf("An error occurred when getting API public hostname: %s", err)
		return "", err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		logrus.Errorf("An error occurred when getting API public hostname: %s", err)
		return "", err
	}
	var jsonData map[string]interface{}
	err = json.Unmarshal(body, &jsonData)
	if err != nil {
		logrus.Errorf("An error occurred when unmarshalling: %s", err)
		return "", err
	}
	hostname = jsonData["issuer"].(string)
	return hostname, nil
}

// getClusterPublicHostnameForOpenshiftV3 is a way to get OpenShift API public DNS/IP
// to be used in OpenShift oAuth provider as baseURL
func getClusterPublicHostnameForOpenshiftV4() (hostname string, err error) {
	http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	client := &http.Client{}
	kubeApi := os.Getenv("KUBERNETES_PORT_443_TCP_ADDR")
	url := "https://" + kubeApi + "/apis/config.openshift.io/v1/infrastructures/cluster"
	req, err := http.NewRequest(http.MethodGet, url, nil)
	file, err := ioutil.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/token")
	if err != nil {
		logrus.Errorf("Failed to locate token file: %s", err)
		return "", err
	}
	token := string(file)

	req.Header = http.Header{
		"Authorization": []string{ "Bearer " + token },
	}
	resp, err := client.Do(req)
	if err != nil {
		logrus.Errorf("An error occurred when getting API public hostname: %s", err)
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode / 100 != 2 {
		message := url + " - " + resp.Status
		logrus.Errorf("An error occurred when getting API public hostname: %s", message)
		return "", errors.New(message)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		logrus.Errorf("An error occurred when getting API public hostname: %s", err)
		return "", err
	}
	var jsonData map[string]interface{}
	err = json.Unmarshal(body, &jsonData)
	if err != nil {
		logrus.Errorf("An error occurred when unmarshalling while getting API public hostname: %s", err)
		return "", err
	}
	switch status := jsonData["status"].(type) {
	case map[string]interface{}:
		hostname = status["apiServerURL"].(string)
	default:	
		logrus.Errorf("An error occurred when unmarshalling while getting API public hostname: %s", body)
		return "", errors.New(string(body))
	}

	return hostname, nil
}

func GenerateProxyJavaOpts(proxyURL string, proxyPort string, nonProxyHosts string, proxyUser string, proxyPassword string) (javaOpts string) {

	proxyHost := strings.TrimLeft(proxyURL, "https://")
	proxyUserPassword := ""
	if len(proxyUser) > 1 && len(proxyPassword) > 1 {
		proxyUserPassword =
			" -Dhttp.proxyUser=" + proxyUser + " -Dhttp.proxyPassword=" + proxyPassword +
				" -Dhttps.proxyUser=" + proxyUser + " -Dhttps.proxyPassword=" + proxyPassword
	}
	javaOpts =
		" -Dhttp.proxyHost=" + proxyHost + " -Dhttp.proxyPort=" + proxyPort +
			" -Dhttps.proxyHost=" + proxyHost + " -Dhttps.proxyPort=" + proxyPort +
			" -Dhttp.nonProxyHosts='" + nonProxyHosts + "'" + proxyUserPassword
	return javaOpts
}

func GenerateProxyEnvs(proxyHost string, proxyPort string, nonProxyHosts string, proxyUser string, proxyPassword string) (proxyUrl string, noProxy string) {
	proxyUrl = proxyHost + ":" + proxyPort
	if len(proxyUser) > 1 && len(proxyPassword) > 1 {
		protocol := strings.Split(proxyHost, "://")[0]
		host := strings.Split(proxyHost, "://")[1]
		proxyUrl = protocol + "://" + proxyUser + ":" + proxyPassword + "@" + host + ":" + proxyPort
	}

	noProxy = strings.Replace(nonProxyHosts, "|", ",", -1)

	return proxyUrl, noProxy
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
