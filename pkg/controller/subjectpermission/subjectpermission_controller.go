package subjectpermission

import (
	"context"
	"fmt"
	"strings"

	managedv1alpha1 "github.com/openshift/rbac-permissions-operator/pkg/apis/managed/v1alpha1"
	controllerutil "github.com/openshift/rbac-permissions-operator/pkg/controller/utils"
	"github.com/openshift/rbac-permissions-operator/pkg/localmetrics"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	k8serr "k8s.io/apimachinery/pkg/api/errors"
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
	opts := client.ListOptions{}
	err = r.client.List(context.TODO(), &opts, clusterRoleList)
	if err != nil {
		reqLogger.Error(err, "Failed to get clusterRoleList")
		return reconcile.Result{}, err
	}

	// get a list of clusterRoleBinding from k8s cluster list
	clusterRoleBindingList := &v1.ClusterRoleBindingList{}
	err = r.client.List(context.TODO(), &opts, clusterRoleBindingList)
	if err != nil {
		reqLogger.Error(err, "Failed to get clusterRoleBindingList")
		return reconcile.Result{}, err
	}

	if len(instance.Spec.ClusterPermissions) != 0 {
		// build a clusterRoleBindingNameList which consists of clusterRoleName-subjectName
		crClusterRoleBindingNameList := buildClusterRoleBindingCRList(instance)

		// check ClusterRoleBindingName
		populateCrClusterRoleBindingNameList := populateClusterRoleBindingNames(crClusterRoleBindingNameList, clusterRoleBindingList)

		var CreatedClusterRoleBindingCount int
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
				if k8serr.IsAlreadyExists(err) {
					continue
				}

				var clusterRoleNames []string
				clusterRoleNames = append(clusterRoleNames, clusterRoleName)

				reqLogger.Error(err, "Failed to create clusterRoleBinding")
				return reconcile.Result{}, err
			}
			// instead of updating the condition just log each successfully created ClusterRoleBinding
			reqLogger.Info(fmt.Sprintf("Successfully created ClusterRoleBinding %s", clusterRoleBindingName))

			// Add Prometheus metrics for this CR
			localmetrics.AddPrometheusMetric(instance)

			//increment counter if successfully created clusterRoleBinding
			CreatedClusterRoleBindingCount++
		}
		if len(populateCrClusterRoleBindingNameList) == CreatedClusterRoleBindingCount && len(populateCrClusterRoleBindingNameList) != 0 {
			// update condition if all ClusterRoleBindings added succesfully
			instance.Status.Conditions = controllerutil.UpdateCondition(instance.Status.Conditions, "Successfully created all ClusterRoleBindings", populateCrClusterRoleBindingNameList, true, managedv1alpha1.SubjectPermissionStateCreated, managedv1alpha1.ClusterRoleBindingCreated)
			err = r.client.Status().Update(context.TODO(), instance)
			if err != nil {
				reqLogger.Error(err, "Failed to update condition in subjectpermission controller when successfully created all cluster role bindings")
				return reconcile.Result{}, err
			}
			return reconcile.Result{}, nil
		}
	}

	// get the NamespaceList
	nsList := &corev1.NamespaceList{}
	err = r.client.List(context.TODO(), &opts, nsList)
	if err != nil {
		reqLogger.Error(err, "Failed to get NamespaceList")
		return reconcile.Result{}, err
	}

	if len(instance.Spec.Permissions) != 0 {
		var CreatedRoleBindingCount int
		var successfullRoleBindingNames []string
		// compile list of allowed namespaces only for this subject permission. NOT a list of subject permissions
		for _, permission := range instance.Spec.Permissions {
			// list of all namespaces in safelist
			safeList := controllerutil.GenerateSafeList(permission.NamespacesAllowedRegex, permission.NamespacesDeniedRegex, nsList)

			var namespaceCount int
			// for each safelisted namespace
			for _, ns := range safeList {
				// get a list of all rolebindings in namespace
				rbList := &v1.RoleBindingList{}
				opts := client.ListOptions{Namespace: ns}
				err = r.client.List(context.TODO(), &opts, rbList)

				// create roleBinding
				roleBinding := controllerutil.NewRoleBindingForClusterRole(permission.ClusterRoleName, instance.Spec.SubjectName, instance.Spec.SubjectKind, ns)

				err := r.client.Create(context.TODO(), roleBinding)
				if err != nil {
					if k8serr.IsAlreadyExists(err) {
						continue
					}

					var permissionsClusterRoleNames []string
					permissionsClusterRoleNames = append(permissionsClusterRoleNames, permission.ClusterRoleName)
					return reconcile.Result{}, err
				}
				successfullRoleBindingNames = append(successfullRoleBindingNames, permission.ClusterRoleName)

				// log each successfully created ClusterRoleBinding
				reqLogger.Info(fmt.Sprintf("Successfully created RoleBinding %s in namespace %s", roleBinding.Name, ns))
				namespaceCount++
			}
			if len(safeList) == namespaceCount {
				//increment roleBindingCounter
				CreatedRoleBindingCount++
			}
		}

		if len(instance.Spec.Permissions) == CreatedRoleBindingCount {
			// update condition if all ClusterRoleBindings added succesfully
			instance.Status.Conditions = controllerutil.UpdateCondition(instance.Status.Conditions, "Successfully created all roleBindings", successfullRoleBindingNames, true, managedv1alpha1.SubjectPermissionStateCreated, managedv1alpha1.RoleBindingCreated)
			err = r.client.Status().Update(context.TODO(), instance)
			if err != nil {
				reqLogger.Error(err, "Failed to update condition in subjectpermission controller when successfully created all cluster role bindings")
				return reconcile.Result{}, err
			}
			return reconcile.Result{}, nil
		}

	}

	return reconcile.Result{}, nil
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

// populateCrClusterRoleNames to see if ClusterRoleName exists as a ClusterRole
// returns list of ClusterRoleNames that do not exist
func populateCrClusterRoleNames(subjectPermission *managedv1alpha1.SubjectPermission, clusterRoleList *v1.ClusterRoleList) []string {
	// we get clusterRoleName by managedv1alpha1.ClusterPermission{}
	crClusterRoleNames := subjectPermission.Spec.ClusterPermissions

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
