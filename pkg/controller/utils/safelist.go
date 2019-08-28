package safelist

import (
	"regexp"

	corev1 "k8s.io/api/core/v1"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
)

var (
	log      = logf.Log.WithName("safelist")
	daLogger = log.WithValues("SafeList", "functions")
)

// 1st pass - allowedRegex
func allowedNamespaceList(allowedRegex string, nsList *corev1.NamespaceList) []string {
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

// 2nd pass - deniedRegex

func safeListAfterDeniedRegex(namespacesDeniedRegex string, safeList []string) []string {
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
