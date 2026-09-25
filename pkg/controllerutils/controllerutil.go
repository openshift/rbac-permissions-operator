package util

import (
	"regexp"

	managedv1alpha1 "github.com/openshift/rbac-permissions-operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// PopulateCrPermissionClusterRoleNames to see if clusterRoleName exists in permission
// returns list of ClusterRoleNames in permissions that do not exist
func PopulateCrPermissionClusterRoleNames(subjectPermission *managedv1alpha1.SubjectPermission, clusterRoleList *v1.ClusterRoleList) []string {
	//permission ClusterRoleName
	permissions := subjectPermission.Spec.Permissions

	var permissionClusterRoleNames []string
	var found bool

	for _, i := range permissions {
		found = false
		for _, a := range clusterRoleList.Items {
			if i.ClusterRoleName == a.Name {
				found = true
			}
		}
		if !found {
			permissionClusterRoleNames = append(permissionClusterRoleNames, i.ClusterRoleName)
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

// NamespaceMatchesPermission reports whether a single namespace name is allowed
// by a permission's allow/deny regex pair, using the same allow-then-deny
// semantics as GenerateSafeList (allow must match, deny must not match). It
// exists so callers that only care about one namespace (e.g. the Namespace
// controller reconciling a single created namespace) can avoid materializing and
// scanning the entire namespace list.
func NamespaceMatchesPermission(namespaceName, allowedRegex, deniedRegex string) bool {
	if !regexp.MustCompile(allowedRegex).MatchString(namespaceName) {
		return false
	}
	if deniedRegex != "" && regexp.MustCompile(deniedRegex).MatchString(namespaceName) {
		return false
	}
	return true
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

	// compile the regex once, not once per namespace
	rp := regexp.MustCompile(allowedRegex)

	// for every namespace on the cluster
	// check that against the allowedRegex in Permission
	for _, namespace := range nsList.Items {
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

	// compile the regex once, not once per namespace
	rp := regexp.MustCompile(namespacesDeniedRegex)

	// for every namespace on SafeList
	// check that against deniedRegex
	for _, namespace := range safeList {
		found := rp.MatchString(namespace)
		// if it does not match then append
		if !found {
			updatedSafeList = append(updatedSafeList, namespace)
		}
	}
	return updatedSafeList

}

// NewRoleBindingForClusterRole creates and returns valid RoleBinding
func NewRoleBindingForClusterRole(clusterRoleName, subjectName, subjectNamespace, subjectKind, namespace string) *v1.RoleBinding {
	roleBinding := &v1.RoleBinding{
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

	if len(subjectNamespace) > 0 {
		roleBinding.Subjects[0].Namespace = subjectNamespace
	}

	return roleBinding
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

// check if namespace exist and NamespacePhase is non terminating
func ValidateNamespace(namespace *corev1.Namespace) bool {
	if namespace.Name != "" && namespace.Status.Phase != corev1.NamespaceTerminating {
		return true
	}
	return false
}
