package namespace_test

import (
	"github.com/golang/mock/gomock"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/openshift/rbac-permissions-operator/pkg/apis/managed/v1alpha1"
	testconst "github.com/openshift/rbac-permissions-operator/pkg/const/test"
	"github.com/openshift/rbac-permissions-operator/pkg/controller/namespace"
	clientmocks "github.com/openshift/rbac-permissions-operator/pkg/util/test/generated/mocks/client"
)

var _ = Describe("Namespace Controller", func() {
	var (
		mockClient                *clientmocks.MockClient
		mockCtrl                  *gomock.Controller
		mockStatusWriter          *clientmocks.MockStatusWriter
		namespaceReconciler       namespace.ReconcileNamespace
		testNamespace             *corev1.Namespace
		testSubjectPermissionList v1alpha1.SubjectPermissionList
	)

	BeforeEach(func() {
		mockCtrl = gomock.NewController(GinkgoT())
		mockClient = clientmocks.NewMockClient(mockCtrl)
		mockStatusWriter = clientmocks.NewMockStatusWriter(mockCtrl)
		namespaceReconciler = namespace.ReconcileNamespace{
			Client: mockClient,
			Scheme: testconst.Scheme,
		}
	})

	Context("Reconciling Namespace", func() {
		BeforeEach(func() {
			testNamespace = &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name:      testconst.TestNamespaceName.Name,
					Namespace: testconst.TestNamespaceName.Namespace,
				},
				Spec:   corev1.NamespaceSpec{},
				Status: corev1.NamespaceStatus{},
			}
		})

		It("Updates the status condition for SubjectPermission if namespace not in safelist", func() {
			gomock.InOrder(
				mockClient.EXPECT().Get(gomock.Any(), testconst.TestNamespaceName, gomock.Any()).Times(1).SetArg(2, *testNamespace),
				mockClient.EXPECT().List(gomock.Any(), gomock.Any()).Times(1).SetArg(1, *testconst.TestNamespaceList),
				mockClient.EXPECT().List(gomock.Any(), gomock.Any()).Times(1).SetArg(1, *testconst.TestSubjectPermissionList),
				mockClient.EXPECT().List(gomock.Any(), gomock.Any(), []client.ListOption{
					client.InNamespace(testconst.TestNamespaceName.Name),
				}).Times(1).SetArg(1, *testconst.TestRoleBindingList),
				mockClient.EXPECT().Status().Return(mockStatusWriter),
				mockStatusWriter.EXPECT().Update(gomock.Any(), gomock.Any()).Times(1).SetArg(1, testconst.TestSubjectPermission),
			)
			_, err := namespaceReconciler.Reconcile(testconst.Context, reconcile.Request{NamespacedName: testconst.TestNamespaceName})
			Expect(err).ToNot(HaveOccurred())
		})

		It("Creates new rolebinding if the namespace is in safelist", func() {
			testSubjectPermissionList = v1alpha1.SubjectPermissionList{
				Items: []v1alpha1.SubjectPermission{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "testSubjectPermission",
							Namespace: "rbac-permissions-operator",
						},
						Spec: v1alpha1.SubjectPermissionSpec{
							SubjectName:        "exampleSubjectName",
							SubjectKind:        "exampleSubjectKind",
							ClusterPermissions: []string{"exampleClusterRoleName", "exampleClusterRoleNameTwo"},
							Permissions: []v1alpha1.Permission{
								{
									ClusterRoleName:        "testClusterRoleName",
									NamespacesAllowedRegex: "test",
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
					},
				},
			}
			gomock.InOrder(
				mockClient.EXPECT().Get(gomock.Any(), testconst.TestNamespaceName, gomock.Any()).Times(1).SetArg(2, *testNamespace),
				mockClient.EXPECT().List(gomock.Any(), gomock.Any()).Times(1).SetArg(1, *testconst.TestNamespaceList),
				mockClient.EXPECT().List(gomock.Any(), gomock.Any()).Times(1).SetArg(1, testSubjectPermissionList),
				mockClient.EXPECT().List(gomock.Any(), gomock.Any(), []client.ListOption{
					client.InNamespace(testconst.TestNamespaceName.Name),
				}).Times(1).SetArg(1, *testconst.TestRoleBindingList),
				mockClient.EXPECT().Create(gomock.Any(), gomock.Any()).SetArg(1, *testconst.TestRoleBinding),
				mockClient.EXPECT().Status().Return(mockStatusWriter),
				mockStatusWriter.EXPECT().Update(gomock.Any(), gomock.Any()).Times(1).SetArg(1, testconst.TestSubjectPermission),
			)
			_, err := namespaceReconciler.Reconcile(testconst.Context, reconcile.Request{NamespacedName: testconst.TestNamespaceName})
			Expect(err).ToNot(HaveOccurred())
		})

	})

	AfterEach(func() {
		mockCtrl.Finish()
	})

})
