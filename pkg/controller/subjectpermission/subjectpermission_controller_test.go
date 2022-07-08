package subjectpermission_test

import (
	"github.com/golang/mock/gomock"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/openshift/rbac-permissions-operator/pkg/apis/managed/v1alpha1"
	testconst "github.com/openshift/rbac-permissions-operator/pkg/const/test"
	"github.com/openshift/rbac-permissions-operator/pkg/controller/subjectpermission"
	clientmocks "github.com/openshift/rbac-permissions-operator/pkg/util/test/generated/mocks/client"
)

var _ = Describe("SubjectPermission Controller", func() {
	var (
		mockClient                  *clientmocks.MockClient
		mockCtrl                    *gomock.Controller
		mockStatusWriter            *clientmocks.MockStatusWriter
		subjectPermissionReconciler subjectpermission.ReconcileSubjectPermission
		testSubjectPermission       v1alpha1.SubjectPermission
		testClusterRoleList         rbacv1.ClusterRoleList
		testNamespaceList           *corev1.NamespaceList
	)

	BeforeEach(func() {
		mockCtrl = gomock.NewController(GinkgoT())
		mockClient = clientmocks.NewMockClient(mockCtrl)
		mockStatusWriter = clientmocks.NewMockStatusWriter(mockCtrl)
		subjectPermissionReconciler = subjectpermission.ReconcileSubjectPermission{
			Client: mockClient,
			Scheme: testconst.Scheme,
		}
	})

	Context("Reconciling SubjectPermission", func() {

		It("Should clear Prometheus metrics for SubjectPermission if the deletion timestamp is not nil", func() {
			testSubjectPermission = v1alpha1.SubjectPermission{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "testSubjectPermission",
					Namespace:         "rbac-permissions-operator",
					DeletionTimestamp: &testconst.TestTime,
				},
			}
			gomock.InOrder(
				mockClient.EXPECT().Get(gomock.Any(), testconst.TestNamespaceName, gomock.Any()).Times(1).SetArg(2, testSubjectPermission),
			)
			_, err := subjectPermissionReconciler.Reconcile(testconst.Context, reconcile.Request{NamespacedName: testconst.TestNamespaceName})
			Expect(err).ToNot(HaveOccurred())
		})

		It("Updates status condition if any ClusterRoleName does not exist as a ClusterRole", func() {
			gomock.InOrder(
				mockClient.EXPECT().Get(gomock.Any(), testconst.TestNamespaceName, gomock.Any()).Times(1).SetArg(2, testconst.TestSubjectPermission),
				mockClient.EXPECT().List(gomock.Any(), gomock.Any()).Times(1).SetArg(1, testconst.TestClusterRoleList),
				mockClient.EXPECT().List(gomock.Any(), gomock.Any()).Times(1).SetArg(1, testconst.TestClusterRoleBindingList),
				mockClient.EXPECT().Status().Return(mockStatusWriter),
				mockStatusWriter.EXPECT().Update(gomock.Any(), gomock.Any()).Times(1).SetArg(1, testconst.TestSubjectPermission),
			)
			_, err := subjectPermissionReconciler.Reconcile(testconst.Context, reconcile.Request{NamespacedName: testconst.TestNamespaceName})
			Expect(err).ToNot(HaveOccurred())
		})

		It("Updates status condition if the clusterolebindings created successfully", func() {
			testClusterRoleList = rbacv1.ClusterRoleList{
				Items: []rbacv1.ClusterRole{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "exampleClusterRoleName",
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "exampleClusterRoleTwo",
						},
					},
				},
			}
			gomock.InOrder(
				mockClient.EXPECT().Get(gomock.Any(), testconst.TestNamespaceName, gomock.Any()).Times(1).SetArg(2, testconst.TestSubjectPermission),
				mockClient.EXPECT().List(gomock.Any(), gomock.Any()).Times(1).SetArg(1, testClusterRoleList),
				mockClient.EXPECT().List(gomock.Any(), gomock.Any()).Times(1).SetArg(1, testconst.TestClusterRoleBindingList),
				mockClient.EXPECT().Create(gomock.Any(), gomock.Any()).Times(2).SetArg(1, testconst.TestClusterRoleBinding),
				mockClient.EXPECT().Status().Return(mockStatusWriter),
				mockStatusWriter.EXPECT().Update(gomock.Any(), gomock.Any()).SetArg(1, testconst.TestSubjectPermission),
			)
			_, err := subjectPermissionReconciler.Reconcile(testconst.Context, reconcile.Request{NamespacedName: testconst.TestNamespaceName})
			Expect(err).ToNot(HaveOccurred())
		})

		It("Should successfully reconcile if there are no ClusterPermissions and no safe namespaces for rolebinding creation", func() {
			testClusterRoleList = rbacv1.ClusterRoleList{
				Items: []rbacv1.ClusterRole{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "exampleClusterRoleName",
						},
					},
				},
			}
			testSubjectPermission = v1alpha1.SubjectPermission{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "testSubjectPermission",
					Namespace: "rbac-permissions-operator",
				},
				Spec: v1alpha1.SubjectPermissionSpec{
					SubjectName:        "exampleSubjectName",
					SubjectKind:        "exampleSubjectKind",
					ClusterPermissions: []string{},
					Permissions: []v1alpha1.Permission{
						{
							ClusterRoleName:        "exampleClusterRoleName",
							NamespacesAllowedRegex: testconst.TestAllowedList,
							NamespacesDeniedRegex:  testconst.TestDeniedList,
						},
						{
							ClusterRoleName:        "testClusterRoleName",
							NamespacesAllowedRegex: "test-namespace",
							NamespacesDeniedRegex:  "",
						},
					},
				},
				Status: v1alpha1.SubjectPermissionStatus{
					Conditions: []v1alpha1.Condition{
						{
							LastTransitionTime: metav1.Now(),
							ClusterRoleNames:   []string{"exampleClusterRoleName"},
							Message:            "exampleMessage",
							Status:             true,
							State:              "exampleState",
						},
					},
				},
			}
			testNamespaceList = &corev1.NamespaceList{
				Items: []corev1.Namespace{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "test",
						},
					},
				},
			}

			gomock.InOrder(
				mockClient.EXPECT().Get(gomock.Any(), testconst.TestNamespaceName, gomock.Any()).Times(1).SetArg(2, testSubjectPermission),
				mockClient.EXPECT().List(gomock.Any(), gomock.Any()).Times(1).SetArg(1, testClusterRoleList),
				mockClient.EXPECT().List(gomock.Any(), gomock.Any()).Times(1).SetArg(1, testconst.TestClusterRoleBindingList),
				mockClient.EXPECT().List(gomock.Any(), gomock.Any()).Times(1).SetArg(1, *testNamespaceList),
			)
			_, err := subjectPermissionReconciler.Reconcile(testconst.Context, reconcile.Request{NamespacedName: testconst.TestNamespaceName})
			Expect(err).ToNot(HaveOccurred())
		})

	})

	AfterEach(func() {
		mockCtrl.Finish()
	})

})
