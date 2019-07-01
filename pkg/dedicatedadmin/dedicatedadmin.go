// Copyright 2018 RedHat
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package dedicatedadmin

import (
	"context"
	"regexp"
	"strings"

	operatorconfig "github.com/openshift/dedicated-admin-operator/config"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
)

var (
	log      = logf.Log.WithName("dedicatedadmin")
	daLogger = log.WithValues("DedicatedAdmin", "functions")
)

// GetAllowedNamespaces returns a list of all namespaces that are allowed based on the input data.  Empty string regex is treated as unset.
func IsNamespaceAllowed(namespacesAllowedRegex string, namespacesDeniedRegex string, allowFirst bool, namespace string) bool {
	if allowFirst && namespacesAllowedRegex != "" {
		// check allow first
		// NOTE if allowed regex is missing nothing is allowed
		allowed, _ := regexp.MatchString(namespacesAllowedRegex, namespace)
		if allowed && namespacesDeniedRegex != "" {
			// it's allowed.  now check that it is not denied.
			denied, _ := regexp.MatchString(namespacesDeniedRegex, namespace)
			if denied {
				// it's denied
				return false
			}
		}
		// it was not denied, return 'allowed' value
		return allowed
	} else {
		// check deny first
		// NOTE if deny regex is missing only the allowed regex applies
		if namespacesDeniedRegex != "" {
			denied, _ := regexp.MatchString(namespacesDeniedRegex, namespace)
			if denied {
				// it's denied
				return false
			}
		}
		// it was not denied, check if it's allowed
		if namespacesAllowedRegex != "" {
			allowed, _ := regexp.MatchString(namespacesAllowedRegex, namespace)
			if allowed {
				// it's allowed
				return true
			}
		}
	}
	// it was not denied or allowed (implies it was denied, default behavior)
	return false
}

// IsBlackListedNamespace matchs a nam,espace against the blacklist
func IsBlackListedNamespace(namespace string, blacklistedNamespaces string) bool {
	for _, blackListedNS := range strings.Split(blacklistedNamespaces, ",") {
		matched, _ := regexp.MatchString(blackListedNS, namespace)
		if matched {
			return true
		}
	}
	return false
}

// GetOperatorConfig gets the operator's configuration from a config map
func GetOperatorConfig(ctx context.Context, k8sClient client.Client) (*corev1.ConfigMap, error) {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      operatorconfig.OperatorConfigMapName,
			Namespace: operatorconfig.OperatorNamespace,
		},
		Data: map[string]string{
			"project_blacklist": "^kube-.*,^openshift-.*,^logging$,^default$,^openshift$,^ops-health-monitoring$,^ops-project-operation-check$,^management-infra$",
		},
	}, nil
}
