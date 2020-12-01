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
	"regexp"
	"strings"
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

// IsDenyListedNamespace matchs a namespace against the denylist
func IsDenyListedNamespace(namespace string, denylistedNamespaces string) bool {
	for _, denyListedNS := range strings.Split(denylistedNamespaces, ",") {
		matched, _ := regexp.MatchString(denyListedNS, namespace)
		if matched {
			return true
		}
	}
	return false
}
