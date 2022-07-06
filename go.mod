module github.com/openshift/rbac-permissions-operator

go 1.16

require (
	github.com/coreos/prometheus-operator v0.38.1-0.20200424145508-7e176fda06cc
	github.com/go-openapi/spec v0.19.5
	github.com/golang/mock v1.4.1
	github.com/onsi/ginkgo v1.16.4
	github.com/onsi/gomega v1.19.0
	github.com/openshift/operator-custom-metrics v0.4.2
	github.com/operator-framework/operator-sdk v0.18.2
	github.com/prometheus/client_golang v1.11.0
	github.com/spf13/pflag v1.0.5
	k8s.io/api v0.22.1
	k8s.io/apimachinery v0.22.1
	k8s.io/client-go v12.0.0+incompatible
	k8s.io/code-generator v0.22.1
	k8s.io/gengo v0.0.0-20201214224949-b6c5ce23f027
	k8s.io/kube-openapi v0.0.0-20210421082810-95288971da7e
	sigs.k8s.io/controller-runtime v0.10.0
)

replace (
	k8s.io/api => k8s.io/api v0.21.1
	k8s.io/apimachinery => k8s.io/apimachinery v0.21.1
	k8s.io/client-go => k8s.io/client-go v0.21.1
	k8s.io/code-generator => k8s.io/code-generator v0.19.4
	k8s.io/kube-openapi => k8s.io/kube-openapi v0.0.0-20210305001622-591a79e4bda7
)
