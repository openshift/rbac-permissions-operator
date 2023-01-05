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
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

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
	err := r.Client.Get(context.TODO(), request.NamespacedName, instance)
	if err != nil {
		if k8serr.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			return ctrl.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return ctrl.Result{}, err
	}

	namespaceList := &corev1.NamespaceList{}
	err = r.Client.List(context.TODO(), namespaceList)
	if err != nil {
		reqLogger.Error(err, "Failed to get namespaceList")
		return ctrl.Result{}, err
	}

	subjectPermissionList := &managedv1alpha1.SubjectPermissionList{}
	err = r.Client.List(context.TODO(), subjectPermissionList)
	if err != nil {
		reqLogger.Error(err, "Failed to get clusterRoleBindingList")
		return ctrl.Result{}, err
	}

	roleBindingList := &v1.RoleBindingList{}
	// request.Name is the instance namespace we are reconciling
	opts := []client.ListOption{
		client.InNamespace(request.Name),
	}
	err = r.Client.List(context.TODO(), roleBindingList, opts...)
	if err != nil {
		reqLogger.Error(err, "Failed to get rolebindingList")
		return ctrl.Result{}, err
	}


	// loop through all subject permissions
	// get namespaces allowed in each permission
	// if our namespace instance is in the safeList, create rolebinding and update condition
	for _, subjectPermission := range subjectPermissionList.Items {
		subPerm := subjectPermission
		var successfulClusterRoleNames []string
		for _, permission := range subPerm.Spec.Permissions {
			successfulClusterRoleNames = append(successfulClusterRoleNames, permission.ClusterRoleName)

			// list of all namespaces in safelist
			safeList := controllerutil.GenerateSafeList(permission.NamespacesAllowedRegex, permission.NamespacesDeniedRegex, namespaceList)
			// if namespace is in safeList, create RoleBinding
			if NamespaceInSlice(instance.Name, safeList) && controllerutil.ValidateNamespace(instance) {

				roleBinding := controllerutil.NewRoleBindingForClusterRole(permission.ClusterRoleName, subPerm.Spec.SubjectName, subPerm.Spec.SubjectKind, instance.Name)
				// if rolebinding is already created in the namespace, continue to next iteration
				if RolebindingInNamespace(roleBinding, roleBindingList) {
					continue
				}

				err := r.Client.Create(context.TODO(), roleBinding)
				if err != nil {
					if k8serr.IsAlreadyExists(err) {
						continue
					}
					failedToCreateRoleBindingMsg := fmt.Sprintf("Failed to create rolebinding %s", roleBinding.Name)
					reqLogger.Error(err, failedToCreateRoleBindingMsg)
					return ctrl.Result{}, err
				}
				roleBindingName := fmt.Sprintf("%s-%s", permission.ClusterRoleName, subjectPermission.Spec.SubjectName)
				reqLogger.Info(fmt.Sprintf("RoleBinding %s created successfully in namespace %s", roleBindingName, instance.Name))
			}
		}
		subPerm.Status.Conditions = controllerutil.UpdateCondition(subPerm.Status.Conditions, "Successfully created all roleBindings", successfulClusterRoleNames, true, managedv1alpha1.SubjectPermissionStateCreated, managedv1alpha1.RoleBindingCreated)
		err = r.Client.Status().Update(context.TODO(), &subPerm)
		if err != nil {
			reqLogger.Error(err, "Failed to update condition in namespace controller when successfully created all cluster role bindings")
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil

}

// check if namespace is in safeList
func NamespaceInSlice(namespace string, safeList []string) bool {
	for _, ns := range safeList {
		if ns == namespace {
			return true
		}
	}
	return false
}

// check if rolebinding is already created in the namespace
func RolebindingInNamespace(rolebinding *v1.RoleBinding, roleBindingList *v1.RoleBindingList) bool {
	list := roleBindingList.Items
	roleBindingName := rolebinding.Name

	for _, rb := range list {
		if rb.Name == roleBindingName {
			return true
		}
	}
	return false
}

// SetupWithManager sets up the controller with the Manager.
func (r *NamespaceReconciler) SetupWithManager(mgr ctrl.Manager) error {

	return ctrl.NewControllerManagedBy(mgr).For(&corev1.Namespace{}).Complete(r)

}
