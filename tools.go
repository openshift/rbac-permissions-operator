//go:build tools
// +build tools

package tools

import (
	// Code generators built at runtime.
	_ "github.com/onsi/ginkgo/ginkgo"
	_ "go.uber.org/mock/mockgen"
	_ "k8s.io/code-generator/cmd/client-gen"
	_ "k8s.io/code-generator/cmd/conversion-gen"
	_ "k8s.io/code-generator/cmd/deepcopy-gen"
	_ "k8s.io/code-generator/cmd/informer-gen"
	_ "k8s.io/code-generator/cmd/lister-gen"
	_ "k8s.io/gengo/args"
	_ "k8s.io/kube-openapi/cmd/openapi-gen"
)
