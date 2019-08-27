package subjectpermission

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	managedv1alpha1 "github.com/openshift/rbac-permissions-operator/pkg/apis/managed/v1alpha1"
	"github.com/openshift/rbac-permissions-operator/pkg/localmetrics"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

var log = logf.Log.WithName("controller_subjectpermission")

/**
* USER ACTION REQUIRED: This is a scaffold file intended for the user to modify with their own Controller
* business logic.  Delete these comments after modifying this file.*
 */

// Add creates a new SubjectPermission Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileSubjectPermission{client: mgr.GetClient(), scheme: mgr.GetScheme()}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("subjectpermission-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource SubjectPermission
	err = c.Watch(&source.Kind{Type: &managedv1alpha1.SubjectPermission{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	// Watch Namespaces because we need to requeue the GroupPermission CR
	// If he permissions in specific namespaces get created or deleted
	err = c.Watch(&source.Kind{Type: &corev1.Namespace{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	return nil
}

// blank assignment to verify that ReconcileSubjectPermission implements reconcile.Reconciler
var _ reconcile.Reconciler = &ReconcileSubjectPermission{}

// ReconcileSubjectPermission reconciles a SubjectPermission object
type ReconcileSubjectPermission struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client client.Client
	scheme *runtime.Scheme
}

// Reconcile reads that state of the cluster for a SubjectPermission object and makes changes based on the state read
// and what is in the SubjectPermission.Spec
// Note:
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *ReconcileSubjectPermission) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	reqLogger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.Info("Reconciling SubjectPermission")

	// Fetch the SubjectPermission instance
	instance := &managedv1alpha1.SubjectPermission{}
	err := r.client.Get(context.TODO(), request.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}

	// The SubjectPermission CR is about to be deleted, so we need to clean up the
	// Prometheus metrics, otherwise there will be stale data exported (for CRs
	// which no longer exist).
	if instance.DeletionTimestamp != nil {
		reqLogger.Info(fmt.Sprintf("Removing Prometheus metrics for SubjectPermission name='%s'", instance.ObjectMeta.GetName()))
		localmetrics.DeletePrometheusMetric(instance)
		return reconcile.Result{}, nil
	}

	// get list of clusterRole on k8s
	clusterRoleList := &v1.ClusterRoleList{}
	opts := client.ListOptions{Namespace: request.Namespace}
	err = r.client.List(context.TODO(), &opts, clusterRoleList)
	if err != nil {
		reqLogger.Error(err, "Failed to get clusterRoleList")
		return reconcile.Result{}, err
	}

	// if crClusterRoleNameList returns list of clusterRoleNames
	crClusterRoleNameList := populateCrClusterRoleNames(instance, clusterRoleList)
	for _, crClusterRoleName := range crClusterRoleNameList {

		// helper func to update the condition of the SubjectPermission object
		instance := updateCondition(instance, crClusterRoleName+" for clusterPermission does not exist", crClusterRoleName, true, "Failed")
		err = r.client.Status().Update(context.TODO(), instance)
		if err != nil {
			reqLogger.Error(err, "Failed to update condition.")
			return reconcile.Result{}, err
		}
	}

	// get a list of clusterRoleBinding from k8s cluster list
	clusterRoleBindingList := &v1.ClusterRoleBindingList{}
	opts = client.ListOptions{Namespace: request.Namespace}
	err = r.client.List(context.TODO(), &opts, clusterRoleBindingList)
	if err != nil {
		reqLogger.Error(err, "Failed to get clusterRoleBindingList")
		return reconcile.Result{}, err
	}

	// build a clusterRoleBindingNameList which consists of clusterRoleName-subjectName
	crClusterRoleBindingNameList := buildClusterRoleBindingCRList(instance)

	// check ClusterRoleBindingName
	populateCrClusterRoleBindingNameList := populateClusterRoleBindingNames(crClusterRoleBindingNameList, clusterRoleBindingList)

	// loop through crClusterRoleBindingNameList
	for _, clusterRoleBindingName := range populateCrClusterRoleBindingNameList {

		// get the clusterRoleName by spliting the clusterRoleBindng name
		clusterRBName := strings.Split(clusterRoleBindingName, "-")
		clusterRoleName := clusterRBName[0]
		subjectName := clusterRBName[1]

		// create a new clusterRoleBinding on cluster
		newCRB := newClusterRoleBinding(clusterRoleName, subjectName, instance.Spec.SubjectKind)
		err := r.client.Create(context.TODO(), newCRB)
		if err != nil {
			// update the condition of the groupPermission object
			instance := updateCondition(instance, "Unable to create ClusterRoleBinding: "+err.Error(), clusterRoleName, true, managedv1alpha1.SubjectPermissionFailed)
			err = r.client.Status().Update(context.TODO(), instance)
			if err != nil {
				reqLogger.Error(err, "Failed to update condition.")
				return reconcile.Result{}, err
			}
			reqLogger.Error(err, "Failed to create clusterRoleBinding")
			return reconcile.Result{}, err
		}
		// helper func to update condition of groupPermission object
		instance := updateCondition(instance, "Successfully created ClusterRoleBinding", clusterRoleName, true, managedv1alpha1.SubjectPermissionCreated)
		err = r.client.Status().Update(context.TODO(), instance)
		if err != nil {
			reqLogger.Error(err, "Failed to update condition.")
			return reconcile.Result{}, err
		}
		// Add Prometheus metrics for this CR
		localmetrics.AddPrometheusMetric(instance)
		return reconcile.Result{}, nil
	}

	// ___________________Permission and Namespace logic_______________________ //

	// update GroupPermisison Cr when namepsace updates
	// by looping through each groupPermission
	groupPermissionList := &managedv1alpha1.SubjectPermissionList{}
	opts = client.ListOptions{Namespace: request.Namespace}
	err = r.client.List(context.TODO(), &opts, groupPermissionList)
	if err != nil {
		reqLogger.Error(err, "Failed to get clusterRoleBindingList")
		return reconcile.Result{}, err
	}

	// for each CR get the namespace and compare with regex
	for _, grouppermission := range groupPermissionList.Items {

		// slice of clusterRoleName that does not exists as a clusterRole
		permissionClusterRoleNameList := populateCrPermissionClusterRoleNames(&grouppermission, clusterRoleList)

		for _, permissionClusterRoleName := range permissionClusterRoleNameList {
			// update condition
			updatedGroupPermission := updateCondition(&grouppermission, permissionClusterRoleName+" for clusterPermission does not exist", permissionClusterRoleName, true, managedv1alpha1.SubjectPermissionFailed)
			err = r.client.Status().Update(context.TODO(), updatedGroupPermission)
			if err != nil {
				reqLogger.Error(err, "Failed to update condition.")
				return reconcile.Result{}, err
			}
		}

		// get the Namespace instance
		ns := &corev1.Namespace{}
		err = r.client.Get(context.TODO(), request.NamespacedName, ns)
		if err != nil {
			if errors.IsNotFound(err) {
				// Object not found, it can be transitioning to the final desired state
				// e. g. deletion or creation still in progress. Return and retry again
				reqLogger.Info("Object not ready")
				return reconcile.Result{}, nil
			}
			// Error reading the object
			reqLogger.Info("Error Getting Namespace")
			return reconcile.Result{}, err
		}

		// get the NamespaceList
		nsList := &corev1.NamespaceList{}
		opts = client.ListOptions{Namespace: request.Namespace}
		err = r.client.List(context.TODO(), &opts, nsList)
		if err != nil {
			reqLogger.Error(err, "Failed to get clusterRoleBindingList")
			return reconcile.Result{}, err
		}

		// loop through each permission of GroupPermission CR
		for _, permission := range grouppermission.Spec.Permissions {
			// apply allow list
			sl := allowedNamespacesList(permission.NamespacesAllowedRegex, nsList)

			// apply deny list
			safeListed := safeListAfterDeniedRegex(permission.NamespacesDeniedRegex, sl)

			// for each safelisted namespace
			for _, ns := range safeListed {
				//create roleBinding
				roleBinding := newRoleBinding(permission.ClusterRoleName, instance.Spec.SubjectName, ns)
				err := r.client.Create(context.TODO(), roleBinding)
				if err != nil {
					// update the condition
					permissionUpdatedCondition := updateCondition(&grouppermission, "Unable to create RoleBinding: "+err.Error(), permission.ClusterRoleName, true, managedv1alpha1.SubjectPermissionFailed)
					err = r.client.Status().Update(context.TODO(), permissionUpdatedCondition)
					if err != nil {
						reqLogger.Error(err, "Failed to update condition.")
						return reconcile.Result{}, err
					}
					reqLogger.Error(err, "Failed to create clusterRoleBinding")
					return reconcile.Result{}, err
				}

				// update condition
				permissionUpdatedCondition := updateCondition(&grouppermission, "Succesfully created RoleBinding", permission.ClusterRoleName, true, managedv1alpha1.SubjectPermissionCreated)
				err = r.client.Status().Update(context.TODO(), permissionUpdatedCondition)
				if err != nil {
					reqLogger.Error(err, "Failed to update condition.")
					return reconcile.Result{}, err
				}
				reqLogger.Error(err, "Failed to create clusterRoleBinding")
				return reconcile.Result{}, err
			}
		}
	}

	return reconcile.Result{}, nil
}

// allowedNamespacesList return a slice of allowed namespaces on cluster given a permission regex
func allowedNamespacesList(namespacesAllowedRegex string, namespaceList *corev1.NamespaceList) []string {
	var matches []string

	// for every namespace on the cluster
	// check that against the allowedRegex in Permission
	for _, namespace := range namespaceList.Items {
		rp := regexp.MustCompile(namespacesAllowedRegex)

		// if namespace on cluster matches with regex, append them to slice
		found := rp.MatchString(namespace.Name)
		if found {
			matches = append(matches, namespace.Name)
		}
	}

	return matches
}

// safeListAfterDeniedRegex takes in slice of allowedNamespaces
// returns slice of namespaces after applying deny regex
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

// newClusterRoleBinding creates and returns ClusterRoleBinding
func newClusterRoleBinding(clusterRoleName, subjectName string, subjectKind string) *v1.ClusterRoleBinding {
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

// newRoleBinding creates and returns RoleBinding
func newRoleBinding(clusterRoleName, groupName, namespace string) *v1.RoleBinding {
	return &v1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      clusterRoleName + "-" + groupName,
			Namespace: namespace,
		},
		Subjects: []v1.Subject{
			{
				Kind: "Group",
				Name: groupName,
			},
		},
		RoleRef: v1.RoleRef{
			Kind: "ClusterRole",
			Name: clusterRoleName,
		},
	}
}

// populateCrClusterRoleNames to see if ClusterRoleName exists as a ClusterRole
// returns list of ClusterRoleNames that do not exist
func populateCrClusterRoleNames(groupPermission *managedv1alpha1.SubjectPermission, clusterRoleList *v1.ClusterRoleList) []string {
	// we get clusterRoleName by managedv1alpha1.ClusterPermission{}
	crClusterRoleNames := groupPermission.Spec.ClusterPermissions

	// items is list of clusterRole on k8s
	onClusterItems := clusterRoleList.Items

	var crClusterRoleNameList []string

	// for every cluster role names on cluster, loop through all crClusterRoleNames, if it doesn't exist then append
	for _, i := range onClusterItems {
		//name := i.Name
		for _, a := range crClusterRoleNames {
			if i.Name != a {
				crClusterRoleNameList = append(crClusterRoleNameList, a)
			}
		}
	}

	return crClusterRoleNameList
}

// populateCrPermissionClusterRoleNames to see if clusterRoleName exists in permission
// returns list of ClusterRoleNames in permissions that do not exist
func populateCrPermissionClusterRoleNames(groupPermission *managedv1alpha1.SubjectPermission, clusterRoleList *v1.ClusterRoleList) []string {
	//permission ClusterRoleName
	permissions := groupPermission.Spec.Permissions

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

// populateClusterRoleBindingNames to see if ClusterRoleBinding exists in k8s ClusterRoleBindlingList
// returns a slice of clusterRoleBindingNames that exists in CR but not in clusterRoleBindingList
func populateClusterRoleBindingNames(clusterRoleBindingNames []string, clusterRoleBindingList *v1.ClusterRoleBindingList) []string {
	var crClusterRoleBindingList []string
	var found bool

	for _, crbName := range clusterRoleBindingNames {
		for _, crBinding := range clusterRoleBindingList.Items {
			if crbName == crBinding.Name {
				found = true
			}
		}
		if !found {
			crClusterRoleBindingList = append(crClusterRoleBindingList, crbName)
		}
		found = false
	}
	return crClusterRoleBindingList
}

// buildClusterRoleBindingCRList which consists of clusterRoleName and subjectName
func buildClusterRoleBindingCRList(clusterPermission *managedv1alpha1.SubjectPermission) []string {
	var clusterRoleBindingNameList []string

	// get instance of SubjectPermission
	for _, a := range clusterPermission.Spec.ClusterPermissions {

		clusterRoleBindingNameList = append(clusterRoleBindingNameList, a+"-"+clusterPermission.Spec.SubjectName)
	}

	return clusterRoleBindingNameList
}

// update the condition of SubjectPermission
func updateCondition(groupPermission *managedv1alpha1.SubjectPermission, message string, clusterRoleName string, status bool, state managedv1alpha1.SubjectPermissionState) *managedv1alpha1.SubjectPermission {
	groupPermissionConditions := groupPermission.Status.Conditions

	// make a new condition
	newCondition := managedv1alpha1.Condition{
		LastTransitionTime: metav1.Now(),
		ClusterRoleName:    clusterRoleName,
		Message:            message,
		Status:             status,
		State:              state,
	}

	// append new condition back to the conditions array
	groupPermission.Status.Conditions = append(groupPermissionConditions, newCondition)

	return groupPermission
}
