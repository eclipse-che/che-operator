module github.com/eclipse-che/che-operator

go 1.15

require (
	github.com/che-incubator/kubernetes-image-puller-operator v0.0.0-20200901231735-f852a5a3ea5c
	github.com/devfile/api/v2 v2.0.0-20210427194344-cd9c30e6aa05
	github.com/devfile/devworkspace-operator v0.2.1-0.20210518134212-698baff5f249
	github.com/golang/mock v1.3.1
	github.com/google/go-cmp v0.5.0
	github.com/openshift/api v3.9.1-0.20190924102528-32369d4db2ad+incompatible
	github.com/operator-framework/api v0.3.20
	github.com/operator-framework/operator-lifecycle-manager v0.0.0-20191115003340-16619cd27fa5
	github.com/operator-framework/operator-sdk v0.15.2
	github.com/prometheus/common v0.7.0
	github.com/sirupsen/logrus v1.4.2
	golang.org/x/net v0.0.0-20200625001655-4c5254603344
	k8s.io/api v0.18.8
	k8s.io/apiextensions-apiserver v0.18.8
	k8s.io/apimachinery v0.18.8
	k8s.io/client-go v12.0.0+incompatible
	k8s.io/utils v0.0.0-20191010214722-8d271d903fe4
	sigs.k8s.io/controller-runtime v0.6.3
	sigs.k8s.io/yaml v1.2.0
)

// Pinned to kubernetes-1.16.2 (apart from k8s.io/api which needs to work with DWO)
replace (
	// We need to import a k8s.io/api that fulfils two things:
	// 1) it is close to v0.18.8 (so that it is compatible with devworkspace operator)
	// 2) is a commit import so that operator-framework doesn't throw up (it itself uses a commit import
	// so we need to replace it with another commit import)
	//
	// So this version is just a commit prior to v0.18.8 (using a commit hash of v0.18.8 itself doesn't work
	// because go mod "helpfully" replaces it with the tag, which is exactly what we don't want.
	// 
	// The joy!
	k8s.io/api => k8s.io/api v0.0.0-20200812052025-e70c4f3b2093
	k8s.io/apiextensions-apiserver => k8s.io/apiextensions-apiserver v0.0.0-20191016113550-5357c4baaf65
	k8s.io/apimachinery => k8s.io/apimachinery v0.0.0-20191004115801-a2eda9f80ab8
	k8s.io/apiserver => k8s.io/apiserver v0.0.0-20191016112112-5190913f932d
	k8s.io/cli-runtime => k8s.io/cli-runtime v0.0.0-20191016114015-74ad18325ed5
	k8s.io/client-go => k8s.io/client-go v0.0.0-20191016111102-bec269661e48
	k8s.io/cloud-provider => k8s.io/cloud-provider v0.0.0-20191016115326-20453efc2458
	k8s.io/cluster-bootstrap => k8s.io/cluster-bootstrap v0.0.0-20191016115129-c07a134afb42
	k8s.io/code-generator => k8s.io/code-generator v0.0.0-20191004115455-8e001e5d1894
	k8s.io/component-base => k8s.io/component-base v0.0.0-20191016111319-039242c015a9
	k8s.io/cri-api => k8s.io/cri-api v0.0.0-20190828162817-608eb1dad4ac
	k8s.io/csi-translation-lib => k8s.io/csi-translation-lib v0.0.0-20191016115521-756ffa5af0bd
	k8s.io/kube-aggregator => k8s.io/kube-aggregator v0.0.0-20191016112429-9587704a8ad4
	k8s.io/kube-controller-manager => k8s.io/kube-controller-manager v0.0.0-20191016114939-2b2b218dc1df
	k8s.io/kube-proxy => k8s.io/kube-proxy v0.0.0-20191016114407-2e83b6f20229
	k8s.io/kube-scheduler => k8s.io/kube-scheduler v0.0.0-20191016114748-65049c67a58b
	k8s.io/kubectl => k8s.io/kubectl v0.0.0-20191016120415-2ed914427d51
	k8s.io/kubelet => k8s.io/kubelet v0.0.0-20191016114556-7841ed97f1b2
	k8s.io/legacy-cloud-providers => k8s.io/legacy-cloud-providers v0.0.0-20191016115753-cf0698c3a16b
	k8s.io/metrics => k8s.io/metrics v0.0.0-20191016113814-3b1a734dba6e
	k8s.io/sample-apiserver => k8s.io/sample-apiserver v0.0.0-20191016112829-06bb3c9d77c9
	sigs.k8s.io/controller-runtime => sigs.k8s.io/controller-runtime v0.4.0
	sigs.k8s.io/yaml => sigs.k8s.io/yaml v1.1.0
)

replace github.com/docker/docker => github.com/moby/moby v0.7.3-0.20190826074503-38ab9da00309 // Required by Helm

replace github.com/openshift/api => github.com/openshift/api v0.0.0-20190924102528-32369d4db2ad // Required until https://github.com/operator-framework/operator-lifecycle-manager/pull/1241 is resolved
