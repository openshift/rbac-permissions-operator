package util

import (
	"regexp"

	managedv1alpha1 "github.com/openshift/rbac-permissions-operator/pkg/apis/managed/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
)

var (
	log      = logf.Log.WithName("safelist")
	daLogger = log.WithValues("SafeList", "functions")
)

// PopulateCrPermissionClusterRoleNames to see if clusterRoleName exists in permission
// returns list of ClusterRoleNames in permissions that do not exist
func PopulateCrPermissionClusterRoleNames(subjectPermission *managedv1alpha1.SubjectPermission, clusterRoleList *v1.ClusterRoleList) []string {
	//permission ClusterRoleName
	permissions := subjectPermission.Spec.Permissions

	var permissionClusterRoleNames []string

	for _, i := range clusterRoleList.Items {
		for _, a := range permissions {
			if i.Name != a.ClusterRoleName {
				permissionClusterRoleNames = append(permissionClusterRoleNames, a.ClusterRoleName)
			}
		}
	}

	return permissionClusterRoleNames
}

// AllowedNamespaceList 1st pass - allowedRegex
func AllowedNamespacesList(allowedRegex string, nsList *corev1.NamespaceList) []string {
	var matches []string

	// for every namespace on the cluster
	// check that against the allowedRegex in Permission
	for _, namespace := range nsList.Items {
		rp := regexp.MustCompile(allowedRegex)

		// if namespace on cluster matches with regex, append them to slice
		found := rp.MatchString(namespace.Name)
		if found {
			matches = append(matches, namespace.Name)
		}
	}

	return matches
}

// SafeListAfterDeniedRegex 2nd pass - deniedRegex
func SafeListAfterDeniedRegex(namespacesDeniedRegex string, safeList []string) []string {
	var updatedSafeList []string

	// for every namespace on SafeList
	// check that against deniedRegex
	for _, namespace := range safeList {
		rp := regexp.MustCompile(namespacesDeniedRegex)

		found := rp.MatchString(namespace)
		// if it does not match then append
		if !found {
			updatedSafeList = append(updatedSafeList, namespace)
		}
	}

	return updatedSafeList

}

// NewRoleBinding creates and returns valid RoleBinding
func NewRoleBinding(clusterRoleName, subjectName, subjectKind, namespace string) *v1.RoleBinding {
	return &v1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      clusterRoleName + "-" + subjectName,
			Namespace: namespace,
		},
		Subjects: []v1.Subject{
			{
				Kind: subjectKind,
				Name: subjectName,
			},
		},
		RoleRef: v1.RoleRef{
			Kind: "ClusterRole",
			Name: clusterRoleName,
		},
	}
}

// UpdateCondition of SubjectPermission
func UpdateCondition(subjectPermission *managedv1alpha1.SubjectPermission, message string, clusterRoleName string, status bool, state managedv1alpha1.SubjectPermissionState) *managedv1alpha1.SubjectPermission {
	groupPermissionConditions := subjectPermission.Status.Conditions

	// make a new condition
	newCondition := managedv1alpha1.Condition{
		LastTransitionTime: metav1.Now(),
		ClusterRoleName:    clusterRoleName,
		Message:            message,
		Status:             status,
		State:              state,
	}

	// append new condition back to the conditions array
	subjectPermission.Status.Conditions = append(groupPermissionConditions, newCondition)

	return subjectPermission
}
