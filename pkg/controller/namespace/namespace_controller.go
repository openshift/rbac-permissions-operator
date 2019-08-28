package namespace

import (
	"context"

	managedv1alpha1 "github.com/openshift/rbac-permissions-operator/pkg/apis/managed/v1alpha1"
	controllerutil "github.com/openshift/rbac-permissions-operator/pkg/controller/utils"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
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
	opts := client.ListOptions{Namespace: request.Namespace}
	err = r.client.List(context.TODO(), &opts, namespaceList)
	if err != nil {
		reqLogger.Error(err, "Failed to get namespaceList")
		return reconcile.Result{}, err
	}

	subjectPermissionList := &managedv1alpha1.SubjectPermissionList{}
	opts = client.ListOptions{Namespace: request.Namespace}
	err = r.client.List(context.TODO(), &opts, subjectPermissionList)
	if err != nil {
		reqLogger.Error(err, "Failed to get clusterRoleBindingList")
		return reconcile.Result{}, err
	}

	clusterRoleList := &v1.ClusterRoleList{}
	opts = client.ListOptions{Namespace: request.Namespace}
	err = r.client.List(context.TODO(), &opts, clusterRoleList)
	if err != nil {
		reqLogger.Error(err, "Failed to get clusterRoleList")
		return reconcile.Result{}, err
	}

	// loop through all subject permissions
	for _, subjectPermission := range subjectPermissionList.Items {
		// loop through all permissions in each
		for _, permission := range subjectPermission.Spec.Permissions {
			// 1st pass, appy allow regex
			sl := controllerutil.AllowedNamespaceList(permission.NamespacesAllowedRegex, namespaceList)

			// 2nd pass, apply deny regex
			safeList := controllerutil.SafeListAfterDeniedRegex(permission.NamespacesDeniedRegex, sl)

			// if namespace is in safeList, create RoleBinding
			if namespaceInSlice(instance.Name, safeList) {
				roleBinding := controllerutil.NewRoleBinding(permission.ClusterRoleName, subjectPermission.Spec.SubjectName, subjectPermission.Spec.SubjectKind, instance.Name)
				err := r.client.Create(context.TODO(), roleBinding)
				if err != nil {
					// update the condition
					permissionUpdatedCondition := controllerutil.UpdateCondition(&subjectPermission, "Unable to create RoleBinding: "+err.Error(), permission.ClusterRoleName, true, managedv1alpha1.SubjectPermissionFailed)
					err = r.client.Status().Update(context.TODO(), permissionUpdatedCondition)

					if err != nil {
						reqLogger.Error(err, "Failed to update condition.")
						return reconcile.Result{}, err
					}
					reqLogger.Error(err, "Failed to create clusterRoleBinding")
					return reconcile.Result{}, err
				}
				permissionUpdatedCondition := controllerutil.UpdateCondition(&subjectPermission, "Succesfully created RoleBinding", permission.ClusterRoleName, true, managedv1alpha1.SubjectPermissionCreated)
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

// check if namespace is in safeList
func namespaceInSlice(namespace string, safeList []string) bool {
	for _, ns := range safeList {
		if ns == namespace {
			return true
		}
	}
	return false
}
