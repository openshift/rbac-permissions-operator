package grouppermission

import (
	"context"
	"fmt"
	"reflect"
	"testing"

	"github.com/openshift/rbac-permissions-operator/pkg/apis"
	"github.com/openshift/rbac-permissions-operator/pkg/apis/managed/v1alpha1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

// create fake client to mock API calls
func newTestReconciler() *ReconcileSubjectPermission {
	return &ReconcileSubjectPermission{
		client: fake.NewFakeClient(),
		scheme: scheme.Scheme,
	}
}

// create a SubjectPermission object so we can resigter it in the fake client
func mockSubjectPermission() *v1alpha1.SubjectPermission {
	return &v1alpha1.SubjectPermission{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "testSubjectPermission",
			Namespace: "rbac-permissions-operator",
		},
		Spec: v1alpha1.SubjectPermissionSpec{
			SubjectName:          "exampleSubjectName",
			ClusterPermissions: []string{"exampleClusterRoleName", "exampleClusterRoleNameTwo"},
		},
		Status: v1alpha1.SubjectPermissionStatus{
			Conditions: []v1alpha1.Condition{
				{
					LastTransitionTime: metav1.Now(),
					ClusterRoleName:    "exampleClusterRoleName",
					Message:            "exampleMessage",
					Status:             true,
					State:              "exampleState",
				},
			},
		},
	}
}

func mockClusterRole() *rbacv1.ClusterRole {
	return &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: "dedicated-admins-cluster",
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{"rbac.authorization.k8s.io"},
				Resources: []string{"clusterrolebindings"},
				Verbs:     []string{"create", "delete", "get", "list"},
			},
		},
	}
}

func mockClusterRoleBinding() *rbacv1.ClusterRoleBinding {
	return &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: "exampleClusterRoleName" + "-" + "exampleSubjectName",
		},
		Subjects: []rbacv1.Subject{
			{
				Kind: "Group",
				Name: "exampleSubjectName",
			},
		},
		RoleRef: rbacv1.RoleRef{
			Kind: "ClusterRole",
			Name: "exampleClusterRoleName",
		},
	}
}

// TestClusterRoleNamesAvailableInCrButNotInCluster tests the populateCrClusterRoleNames function
// given: a SubjectPermissionSpec, an empty k8s ClusterRoleList
// expected: []string with results from SubjectPermissionSpec that is NOT on ClusterRoleList
func TestClusterRoleNamesAvailableInCrButNotInCluster(t *testing.T) {
	ctx := context.TODO()
	reconciler := newTestReconciler()

	// Register operator types with the runtime scheme.
	s := scheme.Scheme

	//Add api to scheme
	if err := apis.AddToScheme(s); err != nil {
		t.Errorf("Unable to add route scheme: (%v)", err)
	}

	err := reconciler.client.Create(ctx, mockClusterRole())
	if err != nil {
		t.Errorf("Couldn't create clusterRole for test: %s", err)
	}

	// get empty ClusterRoleList and give it a namespace
	list := &rbacv1.ClusterRoleList{}
	opts := client.ListOptions{Namespace: ""}

	// create clusterRoleList{}
	err = reconciler.client.List(ctx, &opts, list)
	if err != nil {
		t.Errorf("Couldn't get clusterRoleList for test: %s", err)
	}

	// here is the function we are testing
	// since our mockSubjectPermission() contains 2 ClusterRoleNames
	// that are not on the k8s ClusterRoleList, we expect those to be populated
	tmpList := populateCrClusterRoleNames(mockSubjectPermission(), list)

	// this is the desired result
	resultList := []string{"exampleClusterRoleName", "exampleClusterRoleNameTwo"}

	if len(tmpList) != len(resultList) { // check against an actual number??
		t.Errorf("the length does not match")
	}

	// checks resultList against tmpList, if they are not the same
	// our test fails
	for i, v := range resultList {
		if v != tmpList[i] {
			t.Errorf("got %s, want %s", tmpList, resultList)
		}
	}
}

// TestClusterRoleBindingsAvailableInCrButNotInCluster tests the populateClusterRoleBindingNames function
// given: slice of ClusterRoleBindingNames, k8s ClusterRoleBindingList
// expected: slice of clusterRoleBindings that are available in our CR but NOT in k8s ClusterRoleBindingList
func TestClusterRoleBindingsAvailableInCrButNotInCluster(t *testing.T) {
	// get and populate the k8s ClusterRoleBindingList
	list := &rbacv1.ClusterRoleBindingList{
		Items: []rbacv1.ClusterRoleBinding{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-name-one",
				},
			},
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-name-two",
				},
			},
		},
	}

	// sample CR clusterRoleBindingNames
	clusterRoleBindingNames := []string{"test-name-one", "test-name-three"}

	// since ClusterRoleBindingName contains "test-name-one" and "test-name-three"
	// compare with k8s ClusterRoleBindingList that contains "test-name-one" and "test-name-two"
	// it should return only "test-name-three", which only exists in sample CR clusterRoleBindingNames and NOT on k8s cluster
	tmpList := populateClusterRoleBindingNames(clusterRoleBindingNames, list)

	// desired result
	resultList := []string{"test-name-three"}

	if len(tmpList) != len(resultList) {
		t.Errorf("the length does not match")
	}

	// checks resultList against tmpList, if they are not the same
	// our test fails
	for i, v := range resultList {
		if v != tmpList[i] {
			t.Errorf("got %s, want %s", tmpList, resultList)
		}
	}
}

// TestCreateValidClusterRoleBinding tests the newClusterRoleBinding funtion
// given: clusterRoleName, groupName
// expected: a ClusterRoleBinding that contains the new clusterRoleName and groupName
func TestCreateValidClusterRoleBinding(t *testing.T) {
	ctx := context.TODO()
	reconciler := newTestReconciler()

	// Register operator types with the runtime scheme.
	s := scheme.Scheme

	//Add api to scheme
	if err := apis.AddToScheme(s); err != nil {
		t.Fatalf("Unable to add apis scheme: (%v)", err)
	}

	// creates a groupPermission object
	nerr := reconciler.client.Create(ctx, mockSubjectPermission())
	if nerr != nil {
		t.Errorf("Couldn't create required SubjectPermission object for test: %s", nerr)
	}

	// this is the function we are testing
	// it should return mockClusterRoleBinding() which contains the same clusterRoleName and SubjectName
	newClusterRoleBinding := newClusterRoleBinding("exampleClusterRoleName", "exampleSubjectName")

	// compare the two clusterRoleBinding. They should be exactly the same
	// if not our test fails, log out the difference
	diff := reflect.DeepEqual(*newClusterRoleBinding, *mockClusterRoleBinding())
	if !diff {
		t.Error(diff)
	}
}

// TestValidClusterRoleBindingListCreation tests buildClusterRoleBindingCrList function
// given: SubjectPermission Spec
// expected: slice of ClusterRoleBindingNames which consist of clusterrolename-groupname
func TestValidClusterRoleBindingListCreation(t *testing.T) {

	// this is the function we are testing by using a mock
	buildList := buildClusterRoleBindingCRList(mockSubjectPermission())

	// this is the expected outcome
	result := []string{"exampleClusterRoleName-exampleSubjectName", "exampleClusterRoleNameTwo-exampleSubjectName"}

	// check to see if given is equal to expected
	if len(buildList) != len(result) {
		t.Errorf("the length does not match")
	}
	for i, v := range result {
		if v != buildList[i] {
			t.Errorf("got %s, want %s", buildList, result)
		}
	}
}

// TestSuccesfulConditionUpdateForSubjectPermission tests the updatecondition function.
// given: SubjectPermission object, message, clusterRoleName, status, and state
// expected: an updated SubjectPermission object with the correct updated fields
func TestSuccesfulConditionUpdateForSubjectPermission(t *testing.T) {
	// this is the function we are testing with a mock
	buildCondition := updateCondition(mockSubjectPermission(), "testMessage", "testClusterRoleName", false, "testState")

	// make a map of the result that we want to check mock against
	testMap := make(map[int]v1alpha1.Condition)
	initConOne := v1alpha1.Condition{
		ClusterRoleName: "exampleClusterRoleName",
		Message:         "exampleMessage",
		Status:          true,
		State:           "exampleState",
	}
	initConTwo := v1alpha1.Condition{
		ClusterRoleName: "testClusterRoleName",
		Message:         "testMessage",
		Status:          false,
		State:           "testState",
	}

	testMap[0] = initConOne
	testMap[1] = initConTwo

	// check to see if mock is the same as result
	for i, condition := range testMap {
		if !(testCondition(condition, buildCondition.Status.Conditions[i])) {
			t.Errorf("buildCondition does not match")
		}
	}
}

// helper func for TestUpdateCondition
// condition contains metav1.Time() which we are not testing due to it being auto generate
// therefore we will check every field excluding LastTransitionTime
func testCondition(con0 v1alpha1.Condition, con1 v1alpha1.Condition) bool {
	if con0.ClusterRoleName != con1.ClusterRoleName {
		fmt.Printf("Error, wanted: %s, received: %s\n", con0.ClusterRoleName, con1.ClusterRoleName)
		return false
	}
	if con0.Message != con1.Message {
		fmt.Printf("Error, wanted: %s, received: %s\n", con0.Message, con1.Message)
		return false
	}
	if con0.Status != con1.Status {
		fmt.Printf("Error, wanted: %v, received: %v\n", con0.Status, con1.Status)
		return false
	}
	if con0.State != con1.State {
		fmt.Printf("Error, wanted: %s, received: %s\n", con0.State, con1.State)
		return false
	}
	return true
}
