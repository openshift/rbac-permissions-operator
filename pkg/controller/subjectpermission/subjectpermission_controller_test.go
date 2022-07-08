package subjectpermission_test

import (
	// "fmt"

	"fmt"

	"github.com/golang/mock/gomock"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
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
		subjectPermissionReconciler subjectpermission.ReconcileSubjectPermission
		testSubjectPermission       v1alpha1.SubjectPermission
		testClusterRoleList         rbacv1.ClusterRoleList
		testClusterRoleBindingList  rbacv1.ClusterRoleBindingList
		testNamespaceList           *corev1.NamespaceList
		mockStatusWriter            *clientmocks.MockStatusWriter
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
					Permissions:        []v1alpha1.Permission{},
				},
				Status: v1alpha1.SubjectPermissionStatus{
					Conditions: []v1alpha1.Condition{},
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

		It("Should reconcile successfully if any ClusterRoleName does not exist as a Role", func() {
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
					Conditions: []v1alpha1.Condition{},
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
				mockClient.EXPECT().Status().Return(mockStatusWriter),
				mockStatusWriter.EXPECT().Update(gomock.Any(), gomock.Any()).SetArg(1, testSubjectPermission),
			)
			_, err := subjectPermissionReconciler.Reconcile(testconst.Context, reconcile.Request{NamespacedName: testconst.TestNamespaceName})
			Expect(err).ToNot(HaveOccurred())
		})

		It("Should update the status condition successfully when RoleBindings are created in the safe namespaces", func() {
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
					},
				},
				Status: v1alpha1.SubjectPermissionStatus{
					Conditions: []v1alpha1.Condition{},
				},
			}
			testNamespaceList = &corev1.NamespaceList{
				Items: []corev1.Namespace{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "test",
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "default",
						},
					},
				},
			}

			gomock.InOrder(
				mockClient.EXPECT().Get(gomock.Any(), testconst.TestNamespaceName, gomock.Any()).Times(1).SetArg(2, testSubjectPermission),
				mockClient.EXPECT().List(gomock.Any(), gomock.Any()).Times(1).SetArg(1, testClusterRoleList),
				mockClient.EXPECT().List(gomock.Any(), gomock.Any()).Times(1).SetArg(1, testconst.TestClusterRoleBindingList),
				mockClient.EXPECT().List(gomock.Any(), gomock.Any()).Times(1).SetArg(1, *testNamespaceList),
				mockClient.EXPECT().List(gomock.Any(), gomock.Any(), []client.ListOption{
					client.InNamespace("default"),
				}),
				mockClient.EXPECT().Create(gomock.Any(), gomock.Any()).SetArg(1, *testconst.TestRoleBinding),
				mockClient.EXPECT().Status().Return(mockStatusWriter),
				mockStatusWriter.EXPECT().Update(gomock.Any(), gomock.Any()).SetArg(1, testSubjectPermission),
			)
			_, err := subjectPermissionReconciler.Reconcile(testconst.Context, reconcile.Request{NamespacedName: testconst.TestNamespaceName})
			Expect(err).ToNot(HaveOccurred())
		})

	})

	Context("Reconciling SubjectPermission Controller Failures", func() {

		It("Should fail when not able to Get the SubjectPermission from the namespace", func() {
			testSubjectPermission = v1alpha1.SubjectPermission{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "testSubjectPermission",
					Namespace: "rbac-permissions-operator",
				},
			}
			gomock.InOrder(
				mockClient.EXPECT().Get(gomock.Any(), testconst.TestNamespaceName, gomock.Any()).Times(1).Return(fmt.Errorf("fake error")),
			)
			_, err := subjectPermissionReconciler.Reconcile(testconst.Context, reconcile.Request{NamespacedName: testconst.TestNamespaceName})
			Expect(err).To(HaveOccurred())
			Expect(err).ToNot(BeNil())
		})

		It("Should fail when not able to List the ClusterRoleList", func() {
			testClusterRoleList = rbacv1.ClusterRoleList{}
			gomock.InOrder(
				mockClient.EXPECT().Get(gomock.Any(), testconst.TestNamespaceName, gomock.Any()).SetArg(2, testSubjectPermission),
				mockClient.EXPECT().List(gomock.Any(), gomock.Any()).Times(1).SetArg(1, testClusterRoleList).Return(fmt.Errorf("fake error")),
			)
			_, err := subjectPermissionReconciler.Reconcile(testconst.Context, reconcile.Request{NamespacedName: testconst.TestNamespaceName})
			Expect(err).To(HaveOccurred())
			Expect(err).ToNot(BeNil())
		})

		It("Should fail when not able to List the ClusterRoleBindingList", func() {
			testClusterRoleBindingList = rbacv1.ClusterRoleBindingList{}
			gomock.InOrder(
				mockClient.EXPECT().Get(gomock.Any(), testconst.TestNamespaceName, gomock.Any()).SetArg(2, testSubjectPermission),
				mockClient.EXPECT().List(gomock.Any(), gomock.Any()).Times(1).SetArg(1, testconst.TestClusterRoleList),
				mockClient.EXPECT().List(gomock.Any(), gomock.Any()).Times(1).SetArg(1, testClusterRoleBindingList).Return(fmt.Errorf("fake error")),
			)
			_, err := subjectPermissionReconciler.Reconcile(testconst.Context, reconcile.Request{NamespacedName: testconst.TestNamespaceName})
			Expect(err).To(HaveOccurred())
			Expect(err).ToNot(BeNil())
		})

		It("Should fail when cannot update condition for ClusterRole for ClusterPermission not existing", func() {
			gomock.InOrder(
				mockClient.EXPECT().Get(gomock.Any(), testconst.TestNamespaceName, gomock.Any()).SetArg(2, testconst.TestSubjectPermission),
				mockClient.EXPECT().List(gomock.Any(), gomock.Any()).Times(1).SetArg(1, testconst.TestClusterRoleList),
				mockClient.EXPECT().List(gomock.Any(), gomock.Any()).Times(1).SetArg(1, testconst.TestClusterRoleBindingList),
				mockClient.EXPECT().Status().Return(mockStatusWriter),
				mockStatusWriter.EXPECT().Update(gomock.Any(), gomock.Any()).SetArg(1, testconst.TestSubjectPermission).Return(fmt.Errorf("fake error")),
			)
			_, err := subjectPermissionReconciler.Reconcile(testconst.Context, reconcile.Request{NamespacedName: testconst.TestNamespaceName})
			Expect(err).To(HaveOccurred())
			Expect(err).ToNot(BeNil())
		})

		It("Should fail when cannot update status condition if the clusterolebindings created successfully", func() {
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
				mockClient.EXPECT().Create(gomock.Any(), gomock.Any()).Times(1).SetArg(1, testconst.TestClusterRoleBinding).Return(fmt.Errorf("fake error")),
			)
			_, err := subjectPermissionReconciler.Reconcile(testconst.Context, reconcile.Request{NamespacedName: testconst.TestNamespaceName})
			Expect(err).To(HaveOccurred())
			Expect(err).ToNot(BeNil())
		})

		It("Should fail when cannot update status condition when all clusterolebindings created successfully", func() {
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
				mockStatusWriter.EXPECT().Update(gomock.Any(), gomock.Any()).SetArg(1, testconst.TestSubjectPermission).Return(fmt.Errorf("fake error")),
			)
			_, err := subjectPermissionReconciler.Reconcile(testconst.Context, reconcile.Request{NamespacedName: testconst.TestNamespaceName})
			Expect(err).To(HaveOccurred())
			Expect(err).ToNot(BeNil())
		})

		It("Should fail when cannot List the NamespaceList successfully", func() {
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
					Permissions:        []v1alpha1.Permission{},
				},
				Status: v1alpha1.SubjectPermissionStatus{
					Conditions: []v1alpha1.Condition{},
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
				mockClient.EXPECT().List(gomock.Any(), gomock.Any()).Times(1).SetArg(1, *testNamespaceList).Return(fmt.Errorf("fake error")),
			)
			_, err := subjectPermissionReconciler.Reconcile(testconst.Context, reconcile.Request{NamespacedName: testconst.TestNamespaceName})
			Expect(err).To(HaveOccurred())
			Expect(err).ToNot(BeNil())
		})

		It("Should fail if cannot update status for a ClusterRoleName not existing as a Role", func() {
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
					Conditions: []v1alpha1.Condition{},
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
				mockClient.EXPECT().Status().Return(mockStatusWriter),
				mockStatusWriter.EXPECT().Update(gomock.Any(), gomock.Any()).SetArg(1, testSubjectPermission).Return(fmt.Errorf("fake error")),
			)
			_, err := subjectPermissionReconciler.Reconcile(testconst.Context, reconcile.Request{NamespacedName: testconst.TestNamespaceName})
			Expect(err).To(HaveOccurred())
			Expect(err).ToNot(BeNil())
		})

		It("Should fail if cannot create RoleBindings successfully in the safe namespaces", func() {
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
					},
				},
				Status: v1alpha1.SubjectPermissionStatus{
					Conditions: []v1alpha1.Condition{},
				},
			}
			testNamespaceList = &corev1.NamespaceList{
				Items: []corev1.Namespace{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "test",
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "default",
						},
					},
				},
			}

			gomock.InOrder(
				mockClient.EXPECT().Get(gomock.Any(), testconst.TestNamespaceName, gomock.Any()).Times(1).SetArg(2, testSubjectPermission),
				mockClient.EXPECT().List(gomock.Any(), gomock.Any()).Times(1).SetArg(1, testClusterRoleList),
				mockClient.EXPECT().List(gomock.Any(), gomock.Any()).Times(1).SetArg(1, testconst.TestClusterRoleBindingList),
				mockClient.EXPECT().List(gomock.Any(), gomock.Any()).Times(1).SetArg(1, *testNamespaceList),
				mockClient.EXPECT().List(gomock.Any(), gomock.Any(), []client.ListOption{
					client.InNamespace("default"),
				}),
				mockClient.EXPECT().Create(gomock.Any(), gomock.Any()).SetArg(1, *testconst.TestRoleBinding).Return(fmt.Errorf("fake error")),
			)
			_, err := subjectPermissionReconciler.Reconcile(testconst.Context, reconcile.Request{NamespacedName: testconst.TestNamespaceName})
			Expect(err).To(HaveOccurred())
			Expect(err).ToNot(BeNil())
		})

		It("Should fail if cannot create RoleBindings successfully in the safe namespaces", func() {
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
					},
				},
				Status: v1alpha1.SubjectPermissionStatus{
					Conditions: []v1alpha1.Condition{},
				},
			}
			testNamespaceList = &corev1.NamespaceList{
				Items: []corev1.Namespace{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "test",
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "default",
						},
					},
				},
			}

			gomock.InOrder(
				mockClient.EXPECT().Get(gomock.Any(), testconst.TestNamespaceName, gomock.Any()).Times(1).SetArg(2, testSubjectPermission),
				mockClient.EXPECT().List(gomock.Any(), gomock.Any()).Times(1).SetArg(1, testClusterRoleList),
				mockClient.EXPECT().List(gomock.Any(), gomock.Any()).Times(1).SetArg(1, testconst.TestClusterRoleBindingList),
				mockClient.EXPECT().List(gomock.Any(), gomock.Any()).Times(1).SetArg(1, *testNamespaceList),
				mockClient.EXPECT().List(gomock.Any(), gomock.Any(), []client.ListOption{
					client.InNamespace("default"),
				}),
				mockClient.EXPECT().Create(gomock.Any(), gomock.Any()).SetArg(1, *testconst.TestRoleBinding),
				mockClient.EXPECT().Status().Return(mockStatusWriter),
				mockStatusWriter.EXPECT().Update(gomock.Any(), gomock.Any()).SetArg(1, testSubjectPermission).Return(fmt.Errorf("fake error")),
			)
			_, err := subjectPermissionReconciler.Reconcile(testconst.Context, reconcile.Request{NamespacedName: testconst.TestNamespaceName})
			Expect(err).To(HaveOccurred())
			Expect(err).ToNot(BeNil())
		})

	})

	AfterEach(func() {
		mockCtrl.Finish()
	})

})
