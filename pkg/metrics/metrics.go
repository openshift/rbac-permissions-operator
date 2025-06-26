package metrics

import (
	"fmt"

	managedv1alpha1 "github.com/openshift/rbac-permissions-operator/api/v1alpha1"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/prometheus/client_golang/prometheus"
)

var (
	log = logf.Log.WithName("metrics_subjectpermission")

	// RBACClusterwidePermissions for cluster-wide permissions
	RBACClusterwidePermissions = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "rbac_permissions_operator_cluster_permission",
		Help: "Configured permissions in the cluster-wide scope",
	}, []string{
		"subject_name",
		"subject_permission_name",
		"cluster_permission_name",
		"state",
	})

	// RBACNamespacePermissions for per-namespace permissions
	RBACNamespacePermissions = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "rbac_permissions_operator_namespace_permission",
		Help: "Configured permissions in a per-namespace scope",
	}, []string{
		"subject_name",
		"subject_permission_name",
		"cluster_role_name",
		"namespace_allow",
		"namespace_deny",
		"allow_first",
		"state",
	})

	// MetricsList all metrics exported by this package
	MetricsList = []prometheus.Collector{
		RBACClusterwidePermissions,
		RBACNamespacePermissions,
	}
)

// DeletePrometheusMetric - Helper function to delete both clusterwide and
// namespace permission metrics
func DeletePrometheusMetric(gp *managedv1alpha1.SubjectPermission) {
	deleteRBACClusterPermissionMetric(gp)
	deleteRBACNamespacePermissionMetric(gp)
}

// AddPrometheusMetric - Helper function to add both clusterwide and namespace
// permission metrics
func AddPrometheusMetric(gp *managedv1alpha1.SubjectPermission) {
	addRBACClusterPermissionMetric(gp)
	addRBACNamespacePermissionMetric(gp)
}

// addRBACClusterPermissionMetric - add a SubjectPermission to the exported data
// Iterates through the ClusterPermissions
func addRBACClusterPermissionMetric(gp *managedv1alpha1.SubjectPermission) {
	for _, clusterPermissionName := range gp.Spec.ClusterPermissions {
		RBACClusterwidePermissions.With(prometheus.Labels{
			"subject_name":            gp.Spec.SubjectName,
			"subject_permission_name": gp.ObjectMeta.GetName(),
			"cluster_permission_name": clusterPermissionName,
			"state":                   "1",
		}).Set(1.0)
	}
}

// deleteRBACClusterPermissionMetric - delete a SubjectPermission from the
// exported Prometheus data. Iterates through al the ClusterPermissions
func deleteRBACClusterPermissionMetric(gp *managedv1alpha1.SubjectPermission) {
	var r bool
	for _, clusterPermissionName := range gp.Spec.ClusterPermissions {
		r = RBACClusterwidePermissions.DeleteLabelValues(
			gp.Spec.SubjectName,
			gp.GetName(),
			clusterPermissionName,
			"1",
		)
		// It's possible that we weren't able to delete the metric, so let's log a message to that effect.
		if !r {
			log.Info(fmt.Sprintf("Failed to delete GaugeVec labels: subject_name='%s', subject_permission_name='%s', cluster_permission='%s', state='1'",
				gp.Spec.SubjectName, gp.GetName(), clusterPermissionName))
		}
	}
}

// addRBACNamespacePermissionMetric - add a SubjectPermission to the exported data
// Iterates through the ClusterPermissions
func addRBACNamespacePermissionMetric(gp *managedv1alpha1.SubjectPermission) {

	for _, permission := range gp.Spec.Permissions {
		RBACNamespacePermissions.With(prometheus.Labels{
			"subject_name":            gp.Spec.SubjectName,
			"subject_permission_name": gp.ObjectMeta.GetName(),
			"cluster_role_name":       permission.ClusterRoleName,
			"namespace_allow":         permission.NamespacesAllowedRegex,
			"namespace_deny":          permission.NamespacesDeniedRegex,
			"state":                   "1",
		}).Set(1.0)
	}
}

// deleteRBACNamespacePermissionMetric - delete a SubjectPermission from the
// exported Prometheus data. Iterates through al the Permissions
func deleteRBACNamespacePermissionMetric(gp *managedv1alpha1.SubjectPermission) {
	var r bool

	for _, permission := range gp.Spec.Permissions {
		r = RBACNamespacePermissions.DeleteLabelValues(
			gp.Spec.SubjectName,
			gp.GetName(),
			permission.ClusterRoleName,
			permission.NamespacesAllowedRegex,
			permission.NamespacesDeniedRegex,
			"1",
		)
		// It's possible that we weren't able to delete the metric, so let's log a message to that effect.
		if !r {
			log.Info(fmt.Sprintf("Failed to delete GaugeVec labels: subject_name='%s', subject_permission_name='%s', cluster_permission='%s', state='1'",
				gp.Spec.SubjectName, gp.GetName(), permission.ClusterRoleName))
		}
	}
}

// allowFirstToString translates the boolean value to a "1" or "0" for the
// Prometheus metric
func allowFirstToString(a bool) string {
	if a {
		return "1"
	} else {
		return "0"
	}
}
