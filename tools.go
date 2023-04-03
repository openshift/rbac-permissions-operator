//go:build tools
// +build tools

package tools

import (
	// Code generators built at runtime.
	_ "github.com/golang/mock/mockgen"
	_ "github.com/onsi/ginkgo/ginkgo"
	_ "k8s.io/code-generator/cmd/client-gen"
	_ "k8s.io/code-generator/cmd/conversion-gen"
	_ "k8s.io/code-generator/cmd/deepcopy-gen"
	_ "k8s.io/code-generator/cmd/informer-gen"
	_ "k8s.io/code-generator/cmd/lister-gen"
	_ "k8s.io/gengo/args"
	_ "k8s.io/kube-openapi/cmd/openapi-gen"
)
