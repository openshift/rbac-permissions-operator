/*
Copyright 2022.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package subjectpermission

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	controllerutil "github.com/openshift/rbac-permissions-operator/pkg/controllerutils"
	localmetrics "github.com/openshift/rbac-permissions-operator/pkg/metrics"
	v1 "k8s.io/api/rbac/v1"
	k8serr "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrlutil "sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	managedv1alpha1 "github.com/openshift/rbac-permissions-operator/api/v1alpha1"
)

var log = logf.Log.WithName("controller_subjectpermission")

// SubjectPermissionReconciler reconciles a SubjectPermission object
type SubjectPermissionReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	// Test-friendly flags to disable certain features during testing
	DisableValidation bool
	DisableFinalizers bool
}

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the SubjectPermission object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.11.2/pkg/reconcile
func (r *SubjectPermissionReconciler) Reconcile(ctx context.Context, request ctrl.Request) (ctrl.Result, error) {
	startTime := time.Now()
	result := "success"

	defer func() {
		duration := time.Since(startTime)
		localmetrics.RecordReconcileDuration("subjectpermission", result, duration)
		localmetrics.IncReconcileTotal("subjectpermission", result)
	}()

	reqLogger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.Info("Reconciling SubjectPermission")

	// Fetch the SubjectPermission instance
	instance := &managedv1alpha1.SubjectPermission{}
	err := r.Get(ctx, request.NamespacedName, instance)
	if err != nil {
		if k8serr.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		result = "error"
		localmetrics.IncReconcileErrors("subjectpermission", "fetch")
		return ctrl.Result{}, fmt.Errorf("failed to fetch SubjectPermission: %w", err)
	}

	// Handle validation
	if !r.DisableValidation {
		if done, err := r.handleValidation(ctx, instance, reqLogger); done {
			result = "validation_error"
			return ctrl.Result{}, err
		}
	}

	// Handle finalizer logic
	finalizer := "subjectpermission.managed.openshift.io/finalizer"
	if done, err := r.handleFinalizer(ctx, instance, finalizer, reqLogger, &result); done {
		if err != nil {
			return ctrl.Result{}, err
		}
		// Requeue if finalizer was just added or if deletion is in progress
		return ctrl.Result{Requeue: instance.DeletionTimestamp == nil}, nil
	}

	// Get cluster roles
	clusterRoleList := &v1.ClusterRoleList{}
	if err = r.List(ctx, clusterRoleList); err != nil {
		reqLogger.Error(err, "Failed to get clusterRoleList")
		result = "error"
		localmetrics.IncReconcileErrors("subjectpermission", "list_clusterroles")
		return ctrl.Result{}, fmt.Errorf("failed to list ClusterRoles: %w", err)
	}

	// Validate cluster roles exist
	if done, err := r.validateClusterRolesExist(ctx, instance, clusterRoleList, reqLogger, &result); done {
		return ctrl.Result{}, err
	}

	// Reconcile cluster permissions
	if done, err := r.reconcileClusterPermissions(ctx, instance, reqLogger, &result); done {
		return ctrl.Result{}, err
	}

	// Reconcile namespace permissions
	if done, err := r.reconcileNamespacePermissions(ctx, instance, clusterRoleList, reqLogger); done {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// NewClusterRoleBinding creates and returns ClusterRoleBinding
func NewClusterRoleBinding(clusterRoleName, subjectName string, subjectKind string) *v1.ClusterRoleBinding {
	return &v1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: clusterRoleName + "-" + subjectName,
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

// PopulateCrClusterRoleNames to see if ClusterRoleName exists as a ClusterRole
// returns list of ClusterRoleNames that do not exist
func PopulateCrClusterRoleNames(subjectPermission *managedv1alpha1.SubjectPermission, clusterRoleList *v1.ClusterRoleList) []string {
	crClusterRoleNames := subjectPermission.Spec.ClusterPermissions

	// items is list of clusterRole on k8s
	onClusterItems := clusterRoleList.Items

	var crClusterRoleNameList []string
	var found bool

	// for every CR clusterRoleNames, loop through all k8s lusterRoles, if it doesn't exist then append
	for _, i := range crClusterRoleNames {
		found = false
		for _, a := range onClusterItems {
			if i == a.Name {
				found = true
			}
		}
		if !found {
			crClusterRoleNameList = append(crClusterRoleNameList, i)
		}
	}

	// create a map of all unique elements
	encountered := map[string]bool{}
	for v := range crClusterRoleNameList {
		encountered[crClusterRoleNameList[v]] = true
	}

	// place all keys from map into slice
	result := make([]string, 0, len(encountered))
	for key := range encountered {
		result = append(result, key)
	}

	return result
}

// handleValidation validates the SubjectPermission and updates status if validation fails
func (r *SubjectPermissionReconciler) handleValidation(ctx context.Context, instance *managedv1alpha1.SubjectPermission, reqLogger logr.Logger) (bool, error) {
	if err := r.validateSubjectPermission(instance); err != nil {
		reqLogger.Error(err, "SubjectPermission validation failed")
		localmetrics.IncReconcileErrors("subjectpermission", "validation")
		localmetrics.IncValidationFailures("spec_validation")
		instance.Status.Conditions = controllerutil.UpdateCondition(instance.Status.Conditions, "SubjectPermission validation failed", []string{err.Error()}, true, managedv1alpha1.SubjectPermissionStateFailed, managedv1alpha1.ClusterRoleBindingCreated)
		if updateErr := r.Client.Status().Update(ctx, instance); updateErr != nil {
			reqLogger.Error(updateErr, "Failed to update SubjectPermission status after validation failure")
		}
		return true, fmt.Errorf("SubjectPermission validation failed: %w", err)
	}
	return false, nil
}

// handleFinalizer handles finalizer logic for SubjectPermission deletion
func (r *SubjectPermissionReconciler) handleFinalizer(ctx context.Context, instance *managedv1alpha1.SubjectPermission, finalizer string, reqLogger logr.Logger, result *string) (bool, error) {
	if !r.DisableFinalizers {
		if instance.DeletionTimestamp != nil {
			if ctrlutil.ContainsFinalizer(instance, finalizer) {
				reqLogger.Info("Cleaning up SubjectPermission resources", "name", instance.GetName())
				localmetrics.DeletePrometheusMetric(instance)
				ctrlutil.RemoveFinalizer(instance, finalizer)
				if err := r.Update(ctx, instance); err != nil {
					*result = "error"
					localmetrics.IncReconcileErrors("subjectpermission", "cleanup")
					return true, fmt.Errorf("failed to remove finalizer: %w", err)
				}
			}
			return true, nil
		}
		if !ctrlutil.ContainsFinalizer(instance, finalizer) {
			ctrlutil.AddFinalizer(instance, finalizer)
			if err := r.Update(ctx, instance); err != nil {
				*result = "error"
				localmetrics.IncReconcileErrors("subjectpermission", "finalizer")
				return true, fmt.Errorf("failed to add finalizer: %w", err)
			}
			return true, nil
		}
	} else {
		if instance.DeletionTimestamp != nil {
			reqLogger.Info("Removing Prometheus metrics for SubjectPermission", "name", instance.GetName())
			localmetrics.DeletePrometheusMetric(instance)
			return true, nil
		}
	}
	return false, nil
}

// validateClusterRolesExist checks if all cluster roles referenced in the SubjectPermission exist
func (r *SubjectPermissionReconciler) validateClusterRolesExist(ctx context.Context, instance *managedv1alpha1.SubjectPermission, clusterRoleList *v1.ClusterRoleList, reqLogger logr.Logger, result *string) (bool, error) {
	clusterRoleNamesNotOnCluster := PopulateCrClusterRoleNames(instance, clusterRoleList)
	if len(clusterRoleNamesNotOnCluster) != 0 {
		instance.Status.Conditions = controllerutil.UpdateCondition(instance.Status.Conditions, "ClusterRole for ClusterPermission does not exist", clusterRoleNamesNotOnCluster, true, managedv1alpha1.SubjectPermissionStateFailed, managedv1alpha1.ClusterRoleBindingCreated)
		err := r.Client.Status().Update(ctx, instance)
		if err != nil {
			reqLogger.Error(err, "Failed to update condition in subjectpermission controller when checking ClusterRolenames that do not exist as ClusterRole")
			*result = "error"
			localmetrics.IncReconcileErrors("subjectpermission", "status_update")
			return true, fmt.Errorf("failed to update status for missing ClusterRoles: %w", err)
		}
		*result = "missing_clusterroles"
		return true, nil
	}
	return false, nil
}

// reconcileClusterPermissions creates ClusterRoleBindings for cluster-wide permissions
func (r *SubjectPermissionReconciler) reconcileClusterPermissions(ctx context.Context, instance *managedv1alpha1.SubjectPermission, reqLogger logr.Logger, result *string) (bool, error) {
	var createdClusterRoleBindingCount int
	var createdClusterRoleBinding bool
	var clusterRoleNames []string

	for _, clusterRoleName := range instance.Spec.ClusterPermissions {
		newCRB := NewClusterRoleBinding(clusterRoleName, instance.Spec.SubjectName, instance.Spec.SubjectKind)
		err := r.Create(ctx, newCRB)
		if err != nil {
			if !k8serr.IsAlreadyExists(err) {
				reqLogger.Error(err, "Failed to create ClusterRoleBinding", "clusterRoleName", clusterRoleName, "subjectName", instance.Spec.SubjectName)
				*result = "error"
				localmetrics.IncReconcileErrors("subjectpermission", "create_clusterrolebinding")
				return true, fmt.Errorf("failed to create ClusterRoleBinding for %s: %w", clusterRoleName, err)
			}
		} else {
			clusterRoleBindingName := fmt.Sprintf("%s-%s", clusterRoleName, instance.Spec.SubjectName)
			reqLogger.Info("ClusterRoleBinding created successfully", "name", clusterRoleBindingName, "clusterRoleName", clusterRoleName, "subject", instance.Spec.SubjectName)
			localmetrics.IncResourcesCreated("ClusterRoleBinding", instance.Spec.SubjectName)
			createdClusterRoleBinding = true
		}
		clusterRoleNames = append(clusterRoleNames, clusterRoleName)
		createdClusterRoleBindingCount++
	}

	if createdClusterRoleBinding && len(instance.Spec.ClusterPermissions) == createdClusterRoleBindingCount {
		instance.Status.Conditions = controllerutil.UpdateCondition(instance.Status.Conditions, "Successfully created all ClusterRoleBindings", clusterRoleNames, true, managedv1alpha1.SubjectPermissionStateCreated, managedv1alpha1.ClusterRoleBindingCreated)
		err := r.Client.Status().Update(ctx, instance)
		if err != nil {
			reqLogger.Error(err, "Failed to update condition in subjectpermission controller when successfully created all cluster role bindings")
			return true, err
		}
		return true, nil
	}
	return false, nil
}

// reconcileNamespacePermissions creates RoleBindings for namespace-scoped permissions
func (r *SubjectPermissionReconciler) reconcileNamespacePermissions(ctx context.Context, instance *managedv1alpha1.SubjectPermission, clusterRoleList *v1.ClusterRoleList, reqLogger logr.Logger) (bool, error) {
	if len(instance.Spec.Permissions) == 0 {
		return false, nil
	}

	nsList := &corev1.NamespaceList{}
	err := r.List(ctx, nsList)
	if err != nil {
		reqLogger.Error(err, "Failed to get NamespaceList")
		return true, err
	}

	newNsList := corev1.NamespaceList{}
	for i := range nsList.Items {
		if controllerutil.ValidateNamespace(&nsList.Items[i]) {
			newNsList.Items = append(newNsList.Items, nsList.Items[i])
		} else {
			reqLogger.Info(fmt.Sprintf("Namespace '%s' doesn't exist or in terminating state", nsList.Items[i].Name))
		}
	}

	var createdRoleBindingCount int
	var successfulRoleBindingNames []string

	for _, permission := range instance.Spec.Permissions {
		clusterRoleNamesForPermissionNotOnCluster := controllerutil.PopulateCrPermissionClusterRoleNames(instance, clusterRoleList)
		if len(clusterRoleNamesForPermissionNotOnCluster) != 0 {
			instance.Status.Conditions = controllerutil.UpdateCondition(instance.Status.Conditions, "Role for Permission does not exist", clusterRoleNamesForPermissionNotOnCluster, true, managedv1alpha1.SubjectPermissionStateFailed, managedv1alpha1.RoleBindingCreated)
			err = r.Client.Status().Update(ctx, instance)
			if err != nil {
				reqLogger.Error(err, "Failed to update condition in subjectpermission controller when successfully created all cluster role bindings")
				return true, err
			}
			return true, nil
		}

		safeList := controllerutil.GenerateSafeList(permission.NamespacesAllowedRegex, permission.NamespacesDeniedRegex, &newNsList)
		var namespaceCount int

		for _, ns := range safeList {
			rbList := &v1.RoleBindingList{}
			opts := []client.ListOption{client.InNamespace(ns)}
			_ = r.List(ctx, rbList, opts...)

			roleBinding := controllerutil.NewRoleBindingForClusterRole(permission.ClusterRoleName, instance.Spec.SubjectName, instance.Spec.SubjectNamespace, instance.Spec.SubjectKind, ns)
			err := r.Create(ctx, roleBinding)
			if err != nil {
				if k8serr.IsAlreadyExists(err) {
					continue
				}
				return true, err
			}
			successfulRoleBindingNames = append(successfulRoleBindingNames, permission.ClusterRoleName)
			reqLogger.Info(fmt.Sprintf("Successfully created RoleBinding %s in namespace %s", roleBinding.Name, ns))
			namespaceCount++
		}
		if len(safeList) != 0 && len(safeList) == namespaceCount {
			createdRoleBindingCount++
		}
	}

	if len(instance.Spec.Permissions) == createdRoleBindingCount {
		instance.Status.Conditions = controllerutil.UpdateCondition(instance.Status.Conditions, "Successfully created all roleBindings", successfulRoleBindingNames, true, managedv1alpha1.SubjectPermissionStateCreated, managedv1alpha1.RoleBindingCreated)
		err = r.Client.Status().Update(ctx, instance)
		if err != nil {
			reqLogger.Error(err, "Failed to update condition in subjectpermission controller when successfully created all rolebindings")
			return true, err
		}
		return true, nil
	}

	return false, nil
}

// validateSubjectPermission validates the SubjectPermission spec
func (r *SubjectPermissionReconciler) validateSubjectPermission(sp *managedv1alpha1.SubjectPermission) error {
	// Validate SubjectName
	if strings.TrimSpace(sp.Spec.SubjectName) == "" {
		return fmt.Errorf("subjectName cannot be empty")
	}

	// Validate SubjectKind
	validKinds := []string{"User", "Group", "ServiceAccount"}
	validKind := false
	for _, kind := range validKinds {
		if sp.Spec.SubjectKind == kind {
			validKind = true
			break
		}
	}
	if !validKind {
		return fmt.Errorf("subjectKind must be one of: %s, got: %s", strings.Join(validKinds, ", "), sp.Spec.SubjectKind)
	}

	// Validate ClusterPermissions
	for _, clusterRoleName := range sp.Spec.ClusterPermissions {
		if strings.TrimSpace(clusterRoleName) == "" {
			return fmt.Errorf("clusterRoleName cannot be empty")
		}
	}

	// Validate Permissions regex patterns
	for i, permission := range sp.Spec.Permissions {
		if strings.TrimSpace(permission.ClusterRoleName) == "" {
			return fmt.Errorf("permission[%d].clusterRoleName cannot be empty", i)
		}

		// Validate NamespacesAllowedRegex
		if permission.NamespacesAllowedRegex != "" {
			if _, err := regexp.Compile(permission.NamespacesAllowedRegex); err != nil {
				return fmt.Errorf("invalid namespacesAllowedRegex in permission[%d]: %w", i, err)
			}
		}

		// Validate NamespacesDeniedRegex
		if permission.NamespacesDeniedRegex != "" {
			if _, err := regexp.Compile(permission.NamespacesDeniedRegex); err != nil {
				return fmt.Errorf("invalid namespacesDeniedRegex in permission[%d]: %w", i, err)
			}
		}
	}

	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *SubjectPermissionReconciler) SetupWithManager(mgr ctrl.Manager) error {

	return ctrl.NewControllerManagedBy(mgr).For(&managedv1alpha1.SubjectPermission{}).Complete(r)

}
