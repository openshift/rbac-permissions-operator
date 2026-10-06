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

package namespace

import (
	"context"
	"fmt"

	controllerutil "github.com/openshift/rbac-permissions-operator/pkg/controllerutils"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/rbac/v1"
	k8serr "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	managedv1alpha1 "github.com/openshift/rbac-permissions-operator/api/v1alpha1"
)

var log = logf.Log.WithName("controller_namespace")

// NamespaceReconciler reconciles a Namespace object
type NamespaceReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Namespace object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.11.2/pkg/reconcile
func (r *NamespaceReconciler) Reconcile(ctx context.Context, request ctrl.Request) (ctrl.Result, error) {
	reqLogger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.Info("Reconciling Namespace")

	// Fetch the Namespace instance
	instance := &corev1.Namespace{}
	err := r.Get(ctx, request.NamespacedName, instance)
	if err != nil {
		if k8serr.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			return ctrl.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return ctrl.Result{}, fmt.Errorf("failed to get Namespace %s: %w", request.NamespacedName, err)
	}

	// The controller only reconciles the single namespace named in the request,
	// so there is no need to list every namespace in the cluster. SubjectPermissions
	// are few and are the objects we match against, so we still list them.
	subjectPermissionList := &managedv1alpha1.SubjectPermissionList{}
	err = r.List(ctx, subjectPermissionList)
	if err != nil {
		reqLogger.Error(err, "Failed to get subjectPermissionList")
		return ctrl.Result{}, fmt.Errorf("failed to list SubjectPermissions: %w", err)
	}

	// loop through all subject permissions
	// get namespaces allowed in each permission
	// if our namespace instance matches the permission's regex, create rolebinding and update condition
	for _, subjectPermission := range subjectPermissionList.Items {
		subPerm := subjectPermission
		var successfulClusterRoleNames []string
		bindingCreated := false
		for _, permission := range subPerm.Spec.Permissions {
			successfulClusterRoleNames = append(successfulClusterRoleNames, permission.ClusterRoleName)

			// match this single namespace directly against the permission's regex
			// (same allow-then-deny semantics as GenerateSafeList, without listing
			// or scanning every namespace in the cluster)
			if controllerutil.NamespaceMatchesPermission(instance.Name, permission.NamespacesAllowedRegex, permission.NamespacesDeniedRegex) && controllerutil.ValidateNamespace(instance) {

				roleBinding := controllerutil.NewRoleBindingForClusterRole(permission.ClusterRoleName, subPerm.Spec.SubjectName, subPerm.Spec.SubjectNamespace, subPerm.Spec.SubjectKind, instance.Name)

				// Existence check via a direct API read (RoleBindings are excluded from
				// the cache in main.go, so this Get never builds a RoleBinding informer).
				// On operator restart the Namespace informer replays every existing
				// namespace as a create event; without this check we would attempt a
				// Create for every already-present RoleBinding, producing thousands of
				// avoidable AlreadyExists writes on a large cluster. Mirrors the
				// SubjectPermission controller.
				existing := &v1.RoleBinding{}
				getErr := r.Get(ctx, client.ObjectKey{Namespace: instance.Name, Name: roleBinding.Name}, existing)
				if getErr == nil {
					// Already present: nothing to create, so do not flag a status update
					// (mirrors the previous "already exists -> skip" behavior that avoided
					// unnecessary SubjectPermission reconciliation).
					continue
				}
				if !k8serr.IsNotFound(getErr) {
					reqLogger.Error(getErr, "Failed to check RoleBinding existence")
					return ctrl.Result{}, fmt.Errorf("failed to check RoleBinding existence in namespace %s: %w", instance.Name, getErr)
				}

				err := r.Create(ctx, roleBinding)
				if err != nil {
					// Create is the authoritative, race-safe check; tolerate a concurrent create.
					if k8serr.IsAlreadyExists(err) {
						continue
					}
					reqLogger.Error(err, "Failed to create RoleBinding")
					return ctrl.Result{}, fmt.Errorf("failed to create RoleBinding in namespace %s: %w", instance.Name, err)
				}
				bindingCreated = true
				reqLogger.Info("RoleBinding created successfully", "clusterRole", permission.ClusterRoleName, "namespace", instance.Name)
			}
		}
		// Only update SubjectPermission status when a RoleBinding was actually created
		// to avoid triggering unnecessary reconciliation of the SubjectPermission controller
		if bindingCreated {
			subPerm.Status.Conditions = controllerutil.UpdateCondition(subPerm.Status.Conditions, "Successfully created all roleBindings", successfulClusterRoleNames, true, managedv1alpha1.SubjectPermissionStateCreated, managedv1alpha1.RoleBindingCreated)
			err = r.Client.Status().Update(ctx, &subPerm)
			if err != nil {
				reqLogger.Error(err, "Failed to update condition in namespace controller when successfully created all cluster role bindings")
				return ctrl.Result{}, fmt.Errorf("failed to update SubjectPermission status after creating RoleBindings: %w", err)
			}
		}
	}

	return ctrl.Result{}, nil

}

// CreateOnlyPredicate filters namespace events to only accept create events.
// The namespace controller's purpose is to create RoleBindings for newly
// created namespaces. Update and delete events are irrelevant and would
// cause unnecessary reconciliation storms.
var CreateOnlyPredicate = predicate.Funcs{
	CreateFunc:  func(e event.CreateEvent) bool { return true },
	UpdateFunc:  func(e event.UpdateEvent) bool { return false },
	DeleteFunc:  func(e event.DeleteEvent) bool { return false },
	GenericFunc: func(e event.GenericEvent) bool { return false },
}

// SetupWithManager sets up the controller with the Manager.
func (r *NamespaceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.Namespace{}, builder.WithPredicates(CreateOnlyPredicate)).
		Complete(r)
}
