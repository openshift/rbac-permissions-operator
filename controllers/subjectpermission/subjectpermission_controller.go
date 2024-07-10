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

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	controllerutil "github.com/openshift/rbac-permissions-operator/pkg/controllerutils"
	localmetrics "github.com/openshift/rbac-permissions-operator/pkg/metrics"
	v1 "k8s.io/api/rbac/v1"
	k8serr "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	managedv1alpha1 "github.com/openshift/rbac-permissions-operator/api/v1alpha1"
)

var log = logf.Log.WithName("controller_subjectpermission")

// SubjectPermissionReconciler reconciles a SubjectPermission object
type SubjectPermissionReconciler struct {
	client.Client
	Scheme *runtime.Scheme
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

	reqLogger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.Info("Reconciling SubjectPermission")

	// Fetch the SubjectPermission instance
	instance := &managedv1alpha1.SubjectPermission{}
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

	// The SubjectPermission CR is about to be deleted, so we need to clean up the
	// Prometheus metrics, otherwise there will be stale data exported (for CRs
	// which no longer exist).
	if instance.DeletionTimestamp != nil {
		reqLogger.Info(fmt.Sprintf("Removing Prometheus metrics for SubjectPermission name='%s'", instance.ObjectMeta.GetName()))
		localmetrics.DeletePrometheusMetric(instance)
		return ctrl.Result{}, nil
	}

	// get list of clusterRole on k8s
	clusterRoleList := &v1.ClusterRoleList{}
	err = r.Client.List(context.TODO(), clusterRoleList)
	if err != nil {
		reqLogger.Error(err, "Failed to get clusterRoleList")
		return ctrl.Result{}, err
	}

	// get a list of clusterRoleBinding from k8s cluster list
	clusterRoleBindingList := &v1.ClusterRoleBindingList{}
	err = r.Client.List(context.TODO(), clusterRoleBindingList)
	if err != nil {
		reqLogger.Error(err, "Failed to get clusterRoleBindingList")
		return ctrl.Result{}, err
	}

	// get all ClusterRoleNames that do not exist as ClusterRole
	clusterRoleNamesNotOnCluster := PopulateCrClusterRoleNames(instance, clusterRoleList)
	if len(clusterRoleNamesNotOnCluster) != 0 {
		// update condition if any ClusterRoleName does not exist as a ClusterRole
		instance.Status.Conditions = controllerutil.UpdateCondition(instance.Status.Conditions, "ClusterRole for ClusterPermission does not exist", clusterRoleNamesNotOnCluster, true, managedv1alpha1.SubjectPermissionStateFailed, managedv1alpha1.ClusterRoleBindingCreated)
		err = r.Client.Status().Update(context.TODO(), instance)
		if err != nil {
			reqLogger.Error(err, "Failed to update condition in subjectpermission controller when checking ClusterRolenames that do not exist as ClusterRole")
			return ctrl.Result{}, err
		}
		// exit reconcile, wait for next CR change
		return ctrl.Result{}, nil
	}

	// for every ClusterPermission
	var createdClusterRoleBindingCount int
	var createdClusterRoleBinding bool
	var clusterRoleNames []string
	for _, clusterRoleName := range instance.Spec.ClusterPermissions {
		// create a new ClusterRoleBinding
		newCRB := NewClusterRoleBinding(clusterRoleName, instance.Spec.SubjectName, instance.Spec.SubjectKind)
		err := r.Client.Create(context.TODO(), newCRB)
		if err != nil {
			if !k8serr.IsAlreadyExists(err) {
				reqLogger.Error(err, "Failed to create ClusterRoleBinding")
				return ctrl.Result{}, err
			}
		} else {
			clusterRoleBindingName := fmt.Sprintf("%s-%s", clusterRoleName, instance.Spec.SubjectName)
			reqLogger.Info(fmt.Sprintf("ClusterRoleBinding %s created successfully", clusterRoleBindingName))
			// Created the ClusterRoleBinding, update status later
			createdClusterRoleBinding = true
		}
		// if ClusterRoleBinding created successfully OR ClusterRoleBinding already exists on cluster, add one to counter and append
		clusterRoleNames = append(clusterRoleNames, clusterRoleName)
		createdClusterRoleBindingCount++
	}
	// updateCondition if all ClusterRoleBindings added successfully
	if createdClusterRoleBinding && len(instance.Spec.ClusterPermissions) == createdClusterRoleBindingCount {
		instance.Status.Conditions = controllerutil.UpdateCondition(instance.Status.Conditions, "Successfully created all ClusterRoleBindings", clusterRoleNames, true, managedv1alpha1.SubjectPermissionStateCreated, managedv1alpha1.ClusterRoleBindingCreated)
		err = r.Client.Status().Update(context.TODO(), instance)
		if err != nil {
			reqLogger.Error(err, "Failed to update condition in subjectpermission controller when successfully created all cluster role bindings")
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	// get the NamespaceList
	nsList := &corev1.NamespaceList{}
	err = r.Client.List(context.TODO(), nsList)
	if err != nil {
		reqLogger.Error(err, "Failed to get NamespaceList")
		return ctrl.Result{}, err
	}

	// eliminate terminating and non existing Namespace from the nsList.Items
	newNsList := corev1.NamespaceList{}
	for i := range nsList.Items {
		if controllerutil.ValidateNamespace(&nsList.Items[i]) {
			newNsList.Items = append(newNsList.Items, nsList.Items[i])
		} else {
			reqLogger.Info(fmt.Sprintf("Namespace '%s' doesn't exist or in terminating state", nsList.Items[i].Name))
		}
	}

	if len(instance.Spec.Permissions) != 0 {
		var CreatedRoleBindingCount int
		var successfullRoleBindingNames []string
		// compile list of allowed namespaces only for this subject permission. NOT a list of subject permissions
		for _, permission := range instance.Spec.Permissions {
			// get all ClusterRoleNames that does not exists as RoleNames
			clusterRoleNamesForPermissionNotOnCluster := controllerutil.PopulateCrPermissionClusterRoleNames(instance, clusterRoleList)
			if len(clusterRoleNamesForPermissionNotOnCluster) != 0 {
				// update condition if any ClusterRoleName does not exist as a Role
				instance.Status.Conditions = controllerutil.UpdateCondition(instance.Status.Conditions, "Role for Permission does not exist", clusterRoleNamesForPermissionNotOnCluster, true, managedv1alpha1.SubjectPermissionStateFailed, managedv1alpha1.RoleBindingCreated)
				err = r.Client.Status().Update(context.TODO(), instance)
				if err != nil {
					reqLogger.Error(err, "Failed to update condition in subjectpermission controller when successfully created all cluster role bindings")
					return ctrl.Result{}, err
				}
				// exit reconcile, wait for next CR change
				return ctrl.Result{}, nil
			}

			// list of all namespaces in safelist
			safeList := controllerutil.GenerateSafeList(permission.NamespacesAllowedRegex, permission.NamespacesDeniedRegex, &newNsList)

			var namespaceCount int
			// for each safelisted namespace
			for _, ns := range safeList {
				// get a list of all rolebindings in namespace
				rbList := &v1.RoleBindingList{}
				opts := []client.ListOption{
					client.InNamespace(ns),
				}
				// TODO: Check error
				_ = r.Client.List(context.TODO(), rbList, opts...)

				// create roleBinding
				roleBinding := controllerutil.NewRoleBindingForClusterRole(permission.ClusterRoleName, instance.Spec.SubjectName, instance.Spec.SubjectNamespace, instance.Spec.SubjectKind, ns)

				err := r.Client.Create(context.TODO(), roleBinding)
				if err != nil {
					if k8serr.IsAlreadyExists(err) {
						continue
					}

					return ctrl.Result{}, err
				}
				successfullRoleBindingNames = append(successfullRoleBindingNames, permission.ClusterRoleName)

				// log each successfully created ClusterRoleBinding
				reqLogger.Info(fmt.Sprintf("Successfully created RoleBinding %s in namespace %s", roleBinding.Name, ns))
				namespaceCount++
			}
			if len(safeList) != 0 && len(safeList) == namespaceCount {
				//increment roleBindingCounter
				CreatedRoleBindingCount++
			}
		}

		if len(instance.Spec.Permissions) == CreatedRoleBindingCount {
			// update condition if all RoleBindings added successfully
			instance.Status.Conditions = controllerutil.UpdateCondition(instance.Status.Conditions, "Successfully created all roleBindings", successfullRoleBindingNames, true, managedv1alpha1.SubjectPermissionStateCreated, managedv1alpha1.RoleBindingCreated)
			err = r.Client.Status().Update(context.TODO(), instance)
			if err != nil {
				reqLogger.Error(err, "Failed to update condition in subjectpermission controller when successfully created all rolebindings")
				return ctrl.Result{}, err
			}
			return ctrl.Result{}, nil
		}

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
	result := []string{}
	for key := range encountered {
		result = append(result, key)
	}

	return result
}

// SetupWithManager sets up the controller with the Manager.
func (r *SubjectPermissionReconciler) SetupWithManager(mgr ctrl.Manager) error {

	return ctrl.NewControllerManagedBy(mgr).For(&managedv1alpha1.SubjectPermission{}).Complete(r)

}
