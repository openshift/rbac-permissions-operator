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

	// create a map of all unique elements
	encountered := map[string]bool{}
	for v := range permissionClusterRoleNames {
		encountered[permissionClusterRoleNames[v]] = true
	}

	// place all keys from map into slice
	result := []string{}
	for key := range encountered {
		result = append(result, key)
	}

	return result
}

// GenerateSafeList by 1st checking allow regex then check denied regex
func GenerateSafeList(allowedRegex string, deniedRegex string, nsList *corev1.NamespaceList) []string {
	safeList := allowedNamespacesList(allowedRegex, nsList)

	updatedSafeList := safeListAfterDeniedRegex(deniedRegex, safeList)

	return updatedSafeList

}

// allowedNamespacesList 1st pass - allowedRegex
func allowedNamespacesList(allowedRegex string, nsList *corev1.NamespaceList) []string {
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

// safeListAfterDeniedRegex 2nd pass - deniedRegex
func safeListAfterDeniedRegex(namespacesDeniedRegex string, safeList []string) []string {
	if namespacesDeniedRegex == "" {
		return safeList
	}
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

// NewRoleBindingForClusterRole creates and returns valid RoleBinding
func NewRoleBindingForClusterRole(clusterRoleName, subjectName, subjectKind, namespace string) *v1.RoleBinding {
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
func UpdateCondition(conditions []managedv1alpha1.Condition, message string, clusterRoleNames []string, status bool, state managedv1alpha1.SubjectPermissionState, conditionType managedv1alpha1.SubjectPermissionType) []managedv1alpha1.Condition {
	now := metav1.Now()

	existingCondition := FindRbacCondition(conditions, conditionType)

	// create a map of all unique elements in clusterRoleNames slice
	encountered := map[string]bool{}
	for v := range clusterRoleNames {
		encountered[clusterRoleNames[v]] = true
	}

	// place all keys from map into result slice
	// this prevents the duplication of clusterRoleNames
	result := []string{}
	for key := range encountered {
		result = append(result, key)
	}

	if existingCondition == nil {
		conditions = append(
			conditions, managedv1alpha1.Condition{
				LastTransitionTime: now,
				ClusterRoleNames:   result,
				Message:            message,
				Status:             status,
				State:              state,
				Type:               conditionType,
			},
		)
	} else {
		if existingCondition.Status != status {
			existingCondition.LastTransitionTime = now
		}
		existingCondition.Message = message
		existingCondition.ClusterRoleNames = result
		existingCondition.Status = status
		existingCondition.State = state
	}

	return conditions
}

// FindRbacCondition finds in the condition that has the specified condition type in the given list
// if none exists, then returns nil
func FindRbacCondition(conditions []managedv1alpha1.Condition, conditionType managedv1alpha1.SubjectPermissionType) *managedv1alpha1.Condition {
	for i, condition := range conditions {
		if condition.Type == conditionType {
			return &conditions[i]
		}
	}
	return nil
}

// RoleBindingExists checks if a rolebinding exists in the cluster already
func RoleBindingExists(roleBinding *v1.RoleBinding, rbList *v1.RoleBindingList) bool {
	for _, rb := range rbList.Items {
		if roleBinding.Name == rb.Name {
			return true
		}
	}
	return false
}
