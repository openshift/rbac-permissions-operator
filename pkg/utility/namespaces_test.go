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

package utility

import (
	"testing"
)

func TestIsNamespaceAllowed(t *testing.T) {
	var tests = []struct {
		namespacesAllowedRegex string
		namespacesDeniedRegex  string
		allowFirst             bool
		namespace              string
		valid                  bool
	}{
		// allow first, all regex set
		{".*", "^openshift-.*", true, "somethingelse", true},
		{".*", "^openshift-.*", true, "openshift", true},
		{".*", "^openshift-.*", true, "openshift-monitoring", false},
		{"^openshift-.*", "^openshift-.*", true, "openshift-monitoring", false},
		// allow first, deny regex empty
		{".*", "", true, "somethingelse", true},
		{"^openshift-.*", "", true, "somethingelse", false},
		{"^openshift-.*", "", true, "openshift-monitoring", true},
		// allow first, allow regex empty
		{"", ".*", true, "somethingelse", false},
		{"", "^openshift-.*", true, "somethingelse", false},
		{"", "^openshift-.*", true, "openshift-monitoring", false},
		// allow first, all regex empty
		{"", "", true, "somethingelse", false},

		// deny first, all regex set
		{".*", "^openshift-.*", false, "somethingelse", true},
		{".*", "^openshift-.*", false, "openshift", true},
		{".*", "^openshift-.*", false, "openshift-monitoring", false},
		{"^openshift-.*", "^openshift-.*", false, "openshift-monitoring", false},
		// deny first, deny regex empty
		{".*", "", false, "somethingelse", true},
		{"^openshift-.*", "", false, "somethingelse", false},
		{"^openshift-.*", "", false, "openshift-monitoring", true},
		// deny first, allow regex empty (everything is denied)
		{"", ".*", false, "somethingelse", false},
		{"", "^openshift-.*", false, "somethingelse", false},
		{"", "^openshift-.*", false, "openshift-monitoring", false},
		// deny first, all regex empty
		{"", "", false, "somethingelse", false},

		// more complex examples (i.e. sre)
		{"^(default|openshift.*|kube.*)$", "", true, "default", true},
		{"^(default|openshift.*|kube.*)$", "", true, "openshift-monitoring", true},
		{"^(default|openshift.*|kube.*)$", "", true, "somethingelse", false},
		{"^(default|openshift.*|kube.*)$", "", true, "customer-openshift", false},

		// more complex examples (i.e. customer, allow first)
		{".*", "^(default|openshift.*|kube.*)$", true, "default", false},
		{".*", "^(default|openshift.*|kube.*)$", true, "openshift-monitoring", false},
		{".*", "^(default|openshift.*|kube.*)$", true, "somethingelse", true},
		{".*", "^(default|openshift.*|kube.*)$", true, "customer-openshift", true},

		// more complex examples (i.e. customer, deny first)
		{".*", "^(default|openshift.*|kube.*)$", false, "default", false},
		{".*", "^(default|openshift.*|kube.*)$", false, "openshift-monitoring", false},
		{".*", "^(default|openshift.*|kube.*)$", false, "somethingelse", true},
		{".*", "^(default|openshift.*|kube.*)$", false, "customer-openshift", true},
	}
	for _, test := range tests {
		if IsNamespaceAllowed(test.namespacesAllowedRegex, test.namespacesDeniedRegex, test.allowFirst, test.namespace) != test.valid {
			t.Errorf("FAILURE: IsNamespaceAllowed(%s, %s, %t, %s) = %t, expected = %t", test.namespacesAllowedRegex, test.namespacesDeniedRegex, test.allowFirst, test.namespace, !test.valid, test.valid)
		}
	}
}
