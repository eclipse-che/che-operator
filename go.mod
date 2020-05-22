module github.com/eclipse/che-operator

go 1.12

replace (
	cloud.google.com/go => cloud.google.com/go v0.34.0 // indirect
	github.com/PuerkitoBio/purell => github.com/PuerkitoBio/purell v1.1.0 // indirect
	github.com/PuerkitoBio/urlesc => github.com/PuerkitoBio/urlesc v0.0.0-20170810143723-de5bf2ad4578 // indirect
	github.com/docker/spdystream => github.com/docker/spdystream v0.0.0-20160310174837-449fdfce4d96 // indirect
	github.com/emicklei/go-restful => github.com/emicklei/go-restful v2.8.0+incompatible // indirect
	github.com/ghodss/yaml => github.com/ghodss/yaml v1.0.0 // indirect
	github.com/go-logr/logr => github.com/go-logr/logr v0.1.0 // indirect
	github.com/go-logr/zapr => github.com/go-logr/zapr v0.1.0 // indirect
	github.com/go-openapi/jsonpointer => github.com/go-openapi/jsonpointer v0.0.0-20180322222829-3a0015ad55fa // indirect
	github.com/go-openapi/jsonreference => github.com/go-openapi/jsonreference v0.0.0-20180322222742-3fb327e6747d // indirect
	github.com/go-openapi/spec => github.com/go-openapi/spec v0.0.0-20180801175345-384415f06ee2
	github.com/go-openapi/swag => github.com/go-openapi/swag v0.0.0-20180715190254-becd2f08beaf // indirect
	github.com/gogo/protobuf => github.com/gogo/protobuf v1.2.0 // indirect
	github.com/golang/glog => github.com/golang/glog v0.0.0-20141105023935-44145f04b68c // indirect
	github.com/golang/groupcache => github.com/golang/groupcache v0.0.0-20181024230925-c65c006176ff // indirect
	github.com/google/btree => github.com/google/btree v1.0.0 // indirect
	github.com/google/go-cmp => github.com/google/go-cmp v0.4.0
	github.com/google/gofuzz => github.com/google/gofuzz v0.0.0-20161122191042-44d81051d367 // indirect
	github.com/google/uuid => github.com/google/uuid v1.1.0 // indirect
	github.com/googleapis/gnostic => github.com/googleapis/gnostic v0.2.0 // indirect
	github.com/gregjones/httpcache => github.com/gregjones/httpcache v0.0.0-20180305231024-9cad4c3443a7 // indirect
	github.com/hashicorp/golang-lru => github.com/hashicorp/golang-lru v0.5.0 // indirect
	github.com/imdario/mergo => github.com/imdario/mergo v0.3.6 // indirect
	github.com/json-iterator/go => github.com/json-iterator/go v1.1.5 // indirect
	github.com/mailru/easyjson => github.com/mailru/easyjson v0.0.0-20180823135443-60711f1a8329 // indirect
	github.com/mattbaird/jsonpatch => github.com/mattbaird/jsonpatch v0.0.0-20171005235357-81af80346b1a // indirect
	github.com/modern-go/concurrent => github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 => github.com/modern-go/reflect2 v0.0.0-20180701023420-4b7aa43c6742 // indirect
	github.com/openshift/api => github.com/openshift/api v0.0.0-20190907160150-763878ba9fae
	github.com/operator-framework/operator-sdk => github.com/operator-framework/operator-sdk v0.5.0
	github.com/pborman/uuid => github.com/pborman/uuid v0.0.0-20180906182336-adf5a7427709 // indirect
	github.com/petar/GoLLRB => github.com/petar/GoLLRB v0.0.0-20130427215148-53be0d36a84c // indirect
	github.com/peterbourgon/diskv => github.com/peterbourgon/diskv v2.0.1+incompatible // indirect
	github.com/prometheus/client_golang => github.com/prometheus/client_golang v0.9.2 // indirect
	github.com/prometheus/client_model => github.com/prometheus/client_model v0.0.0-20190129233127-fd36f4220a90 // indirect
	github.com/prometheus/common => github.com/prometheus/common v0.0.0-20190304134840-bf857faf2086
	github.com/prometheus/procfs => github.com/prometheus/procfs v0.0.0-20190225181712-6ed1f7e10411 // indirect
	github.com/sirupsen/logrus => github.com/sirupsen/logrus v1.2.0
	github.com/spf13/pflag => github.com/spf13/pflag v1.0.3 // indirect
	go.uber.org/atomic => go.uber.org/atomic v1.3.2 // indirect
	go.uber.org/multierr => go.uber.org/multierr v1.1.0 // indirect
	go.uber.org/zap => go.uber.org/zap v1.9.1 // indirect
	golang.org/x/crypto => golang.org/x/crypto v0.0.0-20190103213133-ff983b9c42bc // indirect
	golang.org/x/net => golang.org/x/net v0.0.0-20190107210223-45ffb0cd1ba0
	golang.org/x/oauth2 => golang.org/x/oauth2 v0.0.0-20181203162652-d668ce993890 // indirect
	golang.org/x/sys => golang.org/x/sys v0.0.0-20190107173414-20be8e55dc7b // indirect
	golang.org/x/time => golang.org/x/time v0.0.0-20181108054448-85acf8d2951c // indirect
	golang.org/x/tools => golang.org/x/tools v0.0.0-20190107155254-e063def13b29 // indirect
	google.golang.org/appengine => google.golang.org/appengine v1.4.0 // indirect
	gopkg.in/inf.v0 => gopkg.in/inf.v0 v0.9.0 // indirect
	gopkg.in/yaml.v2 => gopkg.in/yaml.v2 v2.2.2 // indirect
	k8s.io/api => k8s.io/api v0.0.0-20181126151915-b503174bad59
	k8s.io/apimachinery => k8s.io/apimachinery v0.0.0-20181126123746-eddba98df674
	k8s.io/client-go => k8s.io/client-go v0.0.0-20181126152608-d082d5923d3c
	k8s.io/code-generator => k8s.io/code-generator v0.0.0-20180823001027-3dcf91f64f63 // indirect
	k8s.io/gengo => k8s.io/gengo v0.0.0-20181113154421-fd15ee9cc2f7 // indirect
	k8s.io/klog => k8s.io/klog v0.1.0 // indirect
	k8s.io/kube-openapi => k8s.io/kube-openapi v0.0.0-20181114233023-0317810137be
	sigs.k8s.io/controller-runtime => sigs.k8s.io/controller-runtime v0.1.8
)

require (
	github.com/PuerkitoBio/purell v1.1.1 // indirect
	github.com/docker/spdystream v0.0.0-00010101000000-000000000000 // indirect
	github.com/elazarl/goproxy v0.0.0-20200426045556-49ad98f6dac1 // indirect
	github.com/go-logr/logr v0.0.0-00010101000000-000000000000 // indirect
	github.com/go-logr/zapr v0.0.0-00010101000000-000000000000 // indirect
	github.com/go-openapi/spec v0.19.3
	github.com/google/go-cmp v0.3.0
	github.com/gregjones/httpcache v0.0.0-00010101000000-000000000000 // indirect
	github.com/imdario/mergo v0.0.0-00010101000000-000000000000 // indirect
	github.com/mattbaird/jsonpatch v0.0.0-00010101000000-000000000000 // indirect
	github.com/onsi/ginkgo v1.12.0 // indirect
	github.com/onsi/gomega v1.10.0 // indirect
	github.com/openshift/api v0.0.0-00010101000000-000000000000
	github.com/operator-framework/operator-sdk v0.0.0-00010101000000-000000000000
	github.com/pborman/uuid v1.2.0 // indirect
	github.com/peterbourgon/diskv v0.0.0-00010101000000-000000000000 // indirect
	github.com/prometheus/common v0.4.1
	github.com/sirupsen/logrus v1.4.2
	golang.org/x/net v0.0.0-20191004110552-13f9640d40b9
	golang.org/x/text v0.3.2 // indirect
	gopkg.in/inf.v0 v0.0.0-00010101000000-000000000000 // indirect
	gopkg.in/yaml.v2 v2.2.8
	k8s.io/api v0.18.2
	k8s.io/apiextensions-apiserver v0.18.2 // indirect
	k8s.io/apimachinery v0.18.2
	k8s.io/client-go v0.18.2
	k8s.io/kube-openapi v0.0.0-20200121204235-bf4fb3bd569c
	sigs.k8s.io/controller-runtime v0.0.0-00010101000000-000000000000
	sigs.k8s.io/testing_frameworks v0.1.2 // indirect
)
