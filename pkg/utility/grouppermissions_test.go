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

	api "github.com/openshift/rbac-permissions-operator/pkg/apis/managed/v1alpha1"
)

func TestGetClusterRoleBindingsForGroupPermissions(t *testing.T) {
	var groupPermissions = []api.GroupPermission{
		{
			Spec: api.GroupPermissionSpec{
				GroupName:          "sre-admins",
				ClusterPermissions: []string{"sre-admins-cluster"},
				Permissions: []api.Permission{
					{
						ClusterRoleName:        "sre-admins-project",
						NamespacesAllowedRegex: "^(default|openshift.*|kube.*)$",
						AllowFirst:             true,
					},
				},
			},
		},
	}

	clusterRoleBindings := GetClusterRoleBindingsForGroupPermissions(groupPermissions)

	for x, clusterRoleBinding := range clusterRoleBindings {
		groupPermission := groupPermissions[x]

		// build table of tests to loop through
		var tests = []struct {
			label    string
			expected string
			found    string
		}{
			{"Group.Name", groupPermission.Spec.GroupName, clusterRoleBinding.Subjects[0].Name},
			{"ClusterRole.Name", groupPermission.Spec.ClusterPermissions[0], clusterRoleBinding.RoleRef.Name},
			{"ClusterRoleBinding.Name", groupPermission.Spec.GroupName + "-" + groupPermission.Spec.ClusterPermissions[0], clusterRoleBinding.ObjectMeta.Name},
		}

		for _, test := range tests {
			if test.expected != test.found {
				t.Errorf("%d: Mismatch for %s.  Expected(%s), Found(%s)", x, test.label, test.expected, test.found)
			}
		}
	}
}
