package subjectpermission_test

import (
	"github.com/golang/mock/gomock"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

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

	})

	AfterEach(func() {
		mockCtrl.Finish()
	})

})
