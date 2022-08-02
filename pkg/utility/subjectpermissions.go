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
	api "github.com/openshift/rbac-permissions-operator/api/v1alpha1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func GetRoleBindingsForNamespace() {

}

func GetRoleBindingsForSubjectPermissions() {

}

func GetClusterRoleBindingsForSubjectPermissions(groupPermissions []api.SubjectPermission) []rbacv1.ClusterRoleBinding {
	var output []rbacv1.ClusterRoleBinding

	for _, groupPermission := range groupPermissions {
		for _, clusterPermission := range groupPermission.Spec.ClusterPermissions {
			crb := rbacv1.ClusterRoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name: groupPermission.Spec.SubjectName + "-" + clusterPermission,
				},
				Subjects: []rbacv1.Subject{
					{
						APIGroup: "rbac.authorization.k8s.io",
						Kind:     "Group",
						Name:     groupPermission.Spec.SubjectName,
					},
				},
				RoleRef: rbacv1.RoleRef{
					APIGroup: "rbac.authorization.k8s.io",
					Kind:     "ClusterRole",
					Name:     clusterPermission,
				},
			}
			output = append(output, crb)
		}
	}

	return output
}
