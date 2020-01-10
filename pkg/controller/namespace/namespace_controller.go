package namespace

import (
	"context"
	"fmt"

	managedv1alpha1 "github.com/openshift/rbac-permissions-operator/pkg/apis/managed/v1alpha1"
	controllerutil "github.com/openshift/rbac-permissions-operator/pkg/controller/utils"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	k8serr "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

var log = logf.Log.WithName("controller_namespace")

/**
* USER ACTION REQUIRED: This is a scaffold file intended for the user to modify with their own Controller
* business logic.  Delete these comments after modifying this file.*
 */

// Add creates a new Namespace Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileNamespace{client: mgr.GetClient(), scheme: mgr.GetScheme()}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("namespace-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource Namespace
	err = c.Watch(&source.Kind{Type: &corev1.Namespace{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	return nil
}

// blank assignment to verify that ReconcileNamespace implements reconcile.Reconciler
var _ reconcile.Reconciler = &ReconcileNamespace{}

// ReconcileNamespace reconciles a Namespace object
type ReconcileNamespace struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client client.Client
	scheme *runtime.Scheme
}

// Reconcile reads that state of the cluster for a Namespace object and makes changes based on the state read
// and what is in the Namespace.Spec
// TODO(user): Modify this Reconcile function to implement your Controller logic.  This example creates
// a Pod as an example
// Note:
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *ReconcileNamespace) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	reqLogger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.Info("Reconciling Namespace")

	// Fetch the Namespace instance
	instance := &corev1.Namespace{}
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

	namespaceList := &corev1.NamespaceList{}
	opts := client.ListOptions{}
	err = r.client.List(context.TODO(), &opts, namespaceList)
	if err != nil {
		reqLogger.Error(err, "Failed to get namespaceList")
		return reconcile.Result{}, err
	}

	subjectPermissionList := &managedv1alpha1.SubjectPermissionList{}
	err = r.client.List(context.TODO(), &opts, subjectPermissionList)
	if err != nil {
		reqLogger.Error(err, "Failed to get clusterRoleBindingList")
		return reconcile.Result{}, err
	}

	roleBindingList := &v1.RoleBindingList{}
	// request.Name is the instance namespace we are reconciling
	opts = client.ListOptions{Namespace: request.Name}
	err = r.client.List(context.TODO(), &opts, roleBindingList)
	if err != nil {
		reqLogger.Error(err, "Failed to get rolebindingList")
		return reconcile.Result{}, err
	}

	// loop through all subject permissions
	// get namespaces allowed in each permission
	// if our namespace instance is in the safeList, create rolebinding and update condition
	for _, subjectPermission := range subjectPermissionList.Items {
		var successfulClusterRoleNames []string
		for _, permission := range subjectPermission.Spec.Permissions {
			successfulClusterRoleNames = append(successfulClusterRoleNames, permission.ClusterRoleName)

			// list of all namespaces in safelist
			safeList := controllerutil.GenerateSafeList(permission.NamespacesAllowedRegex, permission.NamespacesDeniedRegex, namespaceList)
			// if namespace is in safeList, create RoleBinding
			if namespaceInSlice(instance.Name, safeList) {

				roleBinding := controllerutil.NewRoleBindingForClusterRole(permission.ClusterRoleName, subjectPermission.Spec.SubjectName, subjectPermission.Spec.SubjectKind, instance.Name)
				// if rolebinding is already created in the namespace, continue to next iteration
				if rolebindingInNamespace(roleBinding, roleBindingList) {
					continue
				}

				err := r.client.Create(context.TODO(), roleBinding)
				if err != nil {
					if k8serr.IsAlreadyExists(err) {
						continue
					}
					var permissionClusterRoleNames []string
					permissionClusterRoleNames = append(permissionClusterRoleNames, permission.ClusterRoleName)
					failedToCreateRoleBindingMsg := fmt.Sprintf("Failed to create rolebinding %s", roleBinding.Name)
					reqLogger.Error(err, failedToCreateRoleBindingMsg)
					return reconcile.Result{}, err
				}
				roleBindingName := fmt.Sprintf("%s-%s", permission.ClusterRoleName, subjectPermission.Spec.SubjectName)
				reqLogger.Info(fmt.Sprintf("RoleBinding %s created successfully in namespace %s", roleBindingName, instance.Name))
			}
		}
		subjectPermission.Status.Conditions = controllerutil.UpdateCondition(subjectPermission.Status.Conditions, "Successfully created all roleBindings", successfulClusterRoleNames, true, managedv1alpha1.SubjectPermissionStateCreated, managedv1alpha1.RoleBindingCreated)
		err = r.client.Status().Update(context.TODO(), &subjectPermission)
		if err != nil {
			reqLogger.Error(err, "Failed to update condition in namespace controller when successfully created all cluster role bindings")
			return reconcile.Result{}, err
		}
	}

	return reconcile.Result{}, nil
}

// check if namespace is in safeList
func namespaceInSlice(namespace string, safeList []string) bool {
	for _, ns := range safeList {
		if ns == namespace {
			return true
		}
	}
	return false
}

// check if rolebinding is already created in the namespace
func rolebindingInNamespace(rolebinding *v1.RoleBinding, roleBindingList *v1.RoleBindingList) bool {
	list := roleBindingList.Items
	roleBindingName := rolebinding.Name

	for _, rb := range list {
		if rb.Name == roleBindingName {
			return true
		}
	}
	return false
}
