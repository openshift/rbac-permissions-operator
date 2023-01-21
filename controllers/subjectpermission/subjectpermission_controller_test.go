package subjectpermission_test

import (
	"context"
	"fmt"

	"github.com/golang/mock/gomock"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/openshift/rbac-permissions-operator/api/v1alpha1"
	"github.com/openshift/rbac-permissions-operator/controllers/subjectpermission"
	testconst "github.com/openshift/rbac-permissions-operator/pkg/const/test"
	clientmocks "github.com/openshift/rbac-permissions-operator/pkg/util/test/generated/mocks/client"
)

var _ = Describe("SubjectPermission Controller", func() {
	var (
		mockClient                  *clientmocks.MockClient
		mockCtrl                    *gomock.Controller
		subjectPermissionReconciler subjectpermission.SubjectPermissionReconciler
		testSubjectPermission       v1alpha1.SubjectPermission
		testClusterRoleName         string
		testSubjectName             string
		testSubjectKind             string
		testClusterRoleList         rbacv1.ClusterRoleList
		testClusterRoleBindingList  rbacv1.ClusterRoleBindingList
		testNamespaceList           *corev1.NamespaceList
		mockStatusWriter            *clientmocks.MockStatusWriter
	)

	BeforeEach(func() {
		mockCtrl = gomock.NewController(GinkgoT())
		mockClient = clientmocks.NewMockClient(mockCtrl)
		mockStatusWriter = clientmocks.NewMockStatusWriter(mockCtrl)
		subjectPermissionReconciler = subjectpermission.SubjectPermissionReconciler{
			Client: mockClient,
			Scheme: testconst.Scheme,
		}
		testSubjectPermission = testconst.TestSubjectPermission
		testClusterRoleName = "testClusterRole"
		testSubjectName = "testGroupName"
		testSubjectKind = "Group"
		testClusterRoleList = testconst.TestClusterRoleList
		testClusterRoleBindingList = testconst.TestClusterRoleBindingList
	})

	Context("Reconciling SubjectPermission", func() {

		When("ClusterRoleName does not exist as a ClusterRole", func() {
			It("Updates status condition that the ClusterRole for ClusterPermission does not exist", func() {
				gomock.InOrder(
					mockClient.EXPECT().Get(gomock.Any(), testconst.TestNamespaceName, gomock.Any()).Times(1).SetArg(2, testSubjectPermission),
					mockClient.EXPECT().List(gomock.Any(), gomock.Any()).Times(1).SetArg(1, testClusterRoleList),
					mockClient.EXPECT().List(gomock.Any(), gomock.Any()).Times(1).SetArg(1, testClusterRoleBindingList),
					mockClient.EXPECT().Status().Return(mockStatusWriter),
					mockStatusWriter.EXPECT().Update(gomock.Any(), gomock.Any()).Times(1).DoAndReturn(
						func(ctx context.Context, sp *v1alpha1.SubjectPermission, uo ...client.UpdateOption) error {
							Expect(sp.Status.Conditions[1].Message).To(Equal("ClusterRole for ClusterPermission does not exist"))
							Expect(sp.Status.Conditions[1].ClusterRoleNames).To(ContainElement(ContainSubstring("exampleClusterRoleName")))
							Expect(sp.Status.Conditions[1].ClusterRoleNames).To(ContainElement(ContainSubstring("exampleClusterRoleNameTwo")))
							Expect(sp.Status.Conditions[1].Status).To(Equal(true))
							Expect(sp.Status.Conditions[1].State).To(Equal(v1alpha1.SubjectPermissionStateFailed))
							Expect(sp.Status.Conditions[1].Type).To(Equal(v1alpha1.ClusterRoleBindingCreated))
							return nil
						}),
				)
				_, err := subjectPermissionReconciler.Reconcile(testconst.Context, reconcile.Request{NamespacedName: testconst.TestNamespaceName})
				Expect(err).ToNot(HaveOccurred())
			})
		})

		When("ClusterRoleBindings are successfully created", func() {
			BeforeEach(func() {
				testClusterRoleList = rbacv1.ClusterRoleList{
					Items: []rbacv1.ClusterRole{
						{
							ObjectMeta: metav1.ObjectMeta{
								Name: "exampleClusterRoleName",
							},
						},
						{
							ObjectMeta: metav1.ObjectMeta{
								Name: "exampleClusterRoleNameTwo",
							},
						},
					},
				}
			})
			It("Updates status condition for successful creation of ClusterRoleBindings", func() {
				gomock.InOrder(
					mockClient.EXPECT().Get(gomock.Any(), testconst.TestNamespaceName, gomock.Any()).Times(1).SetArg(2, testSubjectPermission),
					mockClient.EXPECT().List(gomock.Any(), gomock.Any()).Times(1).SetArg(1, testClusterRoleList),
					mockClient.EXPECT().List(gomock.Any(), gomock.Any()).Times(1).SetArg(1, testClusterRoleBindingList),
					mockClient.EXPECT().Create(gomock.Any(), gomock.Any()).Times(2).SetArg(1, testconst.TestClusterRoleBinding),
					mockClient.EXPECT().Status().Return(mockStatusWriter),
					mockStatusWriter.EXPECT().Update(gomock.Any(), gomock.Any()).Times(1).DoAndReturn(
						func(ctx context.Context, sp *v1alpha1.SubjectPermission, uo ...client.UpdateOption) error {
							Expect(sp.Status.Conditions[1].Message).To(Equal("Successfully created all ClusterRoleBindings"))
							Expect(sp.Status.Conditions[1].ClusterRoleNames).To(ContainElement(ContainSubstring("exampleClusterRoleName")))
							Expect(sp.Status.Conditions[1].ClusterRoleNames).To(ContainElement(ContainSubstring("exampleClusterRoleNameTwo")))
							Expect(sp.Status.Conditions[1].Status).To(Equal(true))
							Expect(sp.Status.Conditions[1].State).To(Equal(v1alpha1.SubjectPermissionStateCreated))
							Expect(sp.Status.Conditions[1].Type).To(Equal(v1alpha1.ClusterRoleBindingCreated))
							return nil
						}),
				)
				_, err := subjectPermissionReconciler.Reconcile(testconst.Context, reconcile.Request{NamespacedName: testconst.TestNamespaceName})
				Expect(err).ToNot(HaveOccurred())
			})
		})

		When("There are no ClusterPermissions and allowed namespaces for RoleBinding creation", func() {
			BeforeEach(func() {
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
			})
			It("Should successfully reconcile", func() {
				gomock.InOrder(
					mockClient.EXPECT().Get(gomock.Any(), testconst.TestNamespaceName, gomock.Any()).Times(1).SetArg(2, testSubjectPermission),
					mockClient.EXPECT().List(gomock.Any(), gomock.Any()).Times(1).SetArg(1, testClusterRoleList),
					mockClient.EXPECT().List(gomock.Any(), gomock.Any()).Times(1).SetArg(1, testClusterRoleBindingList),
					mockClient.EXPECT().List(gomock.Any(), gomock.Any()).Times(1).SetArg(1, *testNamespaceList),
				)
				_, err := subjectPermissionReconciler.Reconcile(testconst.Context, reconcile.Request{NamespacedName: testconst.TestNamespaceName})
				Expect(err).ToNot(HaveOccurred())
			})
		})

		When("ClusterRoleName does not exist as a Role", func() {
			BeforeEach(func() {
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
								NamespacesAllowedRegex: testconst.TestDefaultAllowedList,
								NamespacesDeniedRegex:  testconst.TestEmptyDeniedList,
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
			})
			It("Should reconcile successfully if any ClusterRoleName does not exist as a Role", func() {
				gomock.InOrder(
					mockClient.EXPECT().Get(gomock.Any(), testconst.TestNamespaceName, gomock.Any()).Times(1).SetArg(2, testSubjectPermission),
					mockClient.EXPECT().List(gomock.Any(), gomock.Any()).Times(1).SetArg(1, testClusterRoleList),
					mockClient.EXPECT().List(gomock.Any(), gomock.Any()).Times(1).SetArg(1, testClusterRoleBindingList),
					mockClient.EXPECT().List(gomock.Any(), gomock.Any()).Times(1).SetArg(1, *testNamespaceList),
					mockClient.EXPECT().Status().Return(mockStatusWriter),
					mockStatusWriter.EXPECT().Update(gomock.Any(), gomock.Any()).Times(1).DoAndReturn(
						func(ctx context.Context, sp *v1alpha1.SubjectPermission, uo ...client.UpdateOption) error {
							Expect(sp.Status.Conditions[0].Message).To(Equal("Role for Permission does not exist"))
							Expect(sp.Status.Conditions[0].ClusterRoleNames[0]).To(Equal("testClusterRoleName"))
							Expect(sp.Status.Conditions[0].Status).To(Equal(true))
							Expect(sp.Status.Conditions[0].State).To(Equal(v1alpha1.SubjectPermissionStateFailed))
							Expect(sp.Status.Conditions[0].Type).To(Equal(v1alpha1.RoleBindingCreated))
							return nil
						}),
				)
				_, err := subjectPermissionReconciler.Reconcile(testconst.Context, reconcile.Request{NamespacedName: testconst.TestNamespaceName})
				Expect(err).ToNot(HaveOccurred())
			})

		})

		When("RoleBindings are created in allowed namespaces", func() {
			BeforeEach(func() {
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
								NamespacesAllowedRegex: testconst.TestDefaultAllowedList,
								NamespacesDeniedRegex:  testconst.TestEmptyDeniedList,
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
			})
			It("Should update the status condition for successful RoleBinding creation", func() {
				gomock.InOrder(
					mockClient.EXPECT().Get(gomock.Any(), testconst.TestNamespaceName, gomock.Any()).Times(1).SetArg(2, testSubjectPermission),
					mockClient.EXPECT().List(gomock.Any(), gomock.Any()).Times(1).SetArg(1, testClusterRoleList),
					mockClient.EXPECT().List(gomock.Any(), gomock.Any()).Times(1).SetArg(1, testClusterRoleBindingList),
					mockClient.EXPECT().List(gomock.Any(), gomock.Any()).Times(1).SetArg(1, *testNamespaceList),
					mockClient.EXPECT().List(gomock.Any(), gomock.Any(), []client.ListOption{
						client.InNamespace("default"),
					}),
					mockClient.EXPECT().Create(gomock.Any(), gomock.Any()).SetArg(1, *testconst.TestRoleBinding),
					mockClient.EXPECT().Status().Return(mockStatusWriter),
					mockStatusWriter.EXPECT().Update(gomock.Any(), gomock.Any()).Times(1).DoAndReturn(
						func(ctx context.Context, sp *v1alpha1.SubjectPermission, uo ...client.UpdateOption) error {
							Expect(sp.Status.Conditions[0].Message).To(Equal("Successfully created all roleBindings"))
							Expect(sp.Status.Conditions[0].ClusterRoleNames[0]).To(Equal("exampleClusterRoleName"))
							Expect(sp.Status.Conditions[0].Status).To(Equal(true))
							Expect(sp.Status.Conditions[0].State).To(Equal(v1alpha1.SubjectPermissionStateCreated))
							Expect(sp.Status.Conditions[0].Type).To(Equal(v1alpha1.RoleBindingCreated))
							return nil
						}),
				)
				_, err := subjectPermissionReconciler.Reconcile(testconst.Context, reconcile.Request{NamespacedName: testconst.TestNamespaceName})
				Expect(err).ToNot(HaveOccurred())
			})
		})
	})

	Context("Reconciling SubjectPermission Controller Failures", func() {
		When("Not able to Get SubjectPermission from a namespace", func() {
			It("Should report failure", func() {
				gomock.InOrder(
					mockClient.EXPECT().Get(gomock.Any(), testconst.TestNamespaceName, gomock.Any()).Times(1).Return(fmt.Errorf("fake error")),
				)
				_, err := subjectPermissionReconciler.Reconcile(testconst.Context, reconcile.Request{NamespacedName: testconst.TestNamespaceName})
				Expect(err).To(HaveOccurred())
			})
		})

		When("Not able to List the ClusterRoleList", func() {
			It("Should report failure", func() {
				testClusterRoleList = rbacv1.ClusterRoleList{}
				gomock.InOrder(
					mockClient.EXPECT().Get(gomock.Any(), testconst.TestNamespaceName, gomock.Any()).SetArg(2, testSubjectPermission),
					mockClient.EXPECT().List(gomock.Any(), gomock.Any()).Times(1).SetArg(1, testClusterRoleList).Return(fmt.Errorf("fake error")),
				)
				_, err := subjectPermissionReconciler.Reconcile(testconst.Context, reconcile.Request{NamespacedName: testconst.TestNamespaceName})
				Expect(err).To(HaveOccurred())
			})
		})

		When("Not able to List the ClusterRoleBindingList", func() {
			BeforeEach(func() {
				testClusterRoleBindingList = rbacv1.ClusterRoleBindingList{}
			})
			It("Should fail when not able to List the ClusterRoleBindingList", func() {
				gomock.InOrder(
					mockClient.EXPECT().Get(gomock.Any(), testconst.TestNamespaceName, gomock.Any()).SetArg(2, testSubjectPermission),
					mockClient.EXPECT().List(gomock.Any(), gomock.Any()).Times(1).SetArg(1, testClusterRoleList),
					mockClient.EXPECT().List(gomock.Any(), gomock.Any()).Times(1).SetArg(1, testClusterRoleBindingList).Return(fmt.Errorf("fake error")),
				)
				_, err := subjectPermissionReconciler.Reconcile(testconst.Context, reconcile.Request{NamespacedName: testconst.TestNamespaceName})
				Expect(err).To(HaveOccurred())
			})
		})

		When("Not able to update condition for ClusterRoleName not existing as a ClusterRole", func() {
			It("Should report failure", func() {
				gomock.InOrder(
					mockClient.EXPECT().Get(gomock.Any(), testconst.TestNamespaceName, gomock.Any()).SetArg(2, testSubjectPermission),
					mockClient.EXPECT().List(gomock.Any(), gomock.Any()).Times(1).SetArg(1, testClusterRoleList),
					mockClient.EXPECT().List(gomock.Any(), gomock.Any()).Times(1).SetArg(1, testClusterRoleBindingList),
					mockClient.EXPECT().Status().Return(mockStatusWriter),
					mockStatusWriter.EXPECT().Update(gomock.Any(), gomock.Any()).SetArg(1, testSubjectPermission).Return(fmt.Errorf("fake error")),
				)
				_, err := subjectPermissionReconciler.Reconcile(testconst.Context, reconcile.Request{NamespacedName: testconst.TestNamespaceName})
				Expect(err).To(HaveOccurred())
			})
		})

		When("Not able to update condition for successful ClusterRoleBindings creation", func() {
			BeforeEach(func() {
				testClusterRoleList = rbacv1.ClusterRoleList{
					Items: []rbacv1.ClusterRole{
						{
							ObjectMeta: metav1.ObjectMeta{
								Name: "exampleClusterRoleName",
							},
						},
						{
							ObjectMeta: metav1.ObjectMeta{
								Name: "exampleClusterRoleNameTwo",
							},
						},
					},
				}
			})
			It("Should report failure", func() {
				gomock.InOrder(
					mockClient.EXPECT().Get(gomock.Any(), testconst.TestNamespaceName, gomock.Any()).Times(1).SetArg(2, testSubjectPermission),
					mockClient.EXPECT().List(gomock.Any(), gomock.Any()).Times(1).SetArg(1, testClusterRoleList),
					mockClient.EXPECT().List(gomock.Any(), gomock.Any()).Times(1).SetArg(1, testClusterRoleBindingList),
					mockClient.EXPECT().Create(gomock.Any(), gomock.Any()).Times(1).SetArg(1, testconst.TestClusterRoleBinding).Return(fmt.Errorf("fake error")),
				)
				_, err := subjectPermissionReconciler.Reconcile(testconst.Context, reconcile.Request{NamespacedName: testconst.TestNamespaceName})
				Expect(err).To(HaveOccurred())
			})
		})

		When("Not able to update status for all ClusterRoleBindings created successfully", func() {
			BeforeEach(func() {
				testClusterRoleList = rbacv1.ClusterRoleList{
					Items: []rbacv1.ClusterRole{
						{
							ObjectMeta: metav1.ObjectMeta{
								Name: "exampleClusterRoleName",
							},
						},
						{
							ObjectMeta: metav1.ObjectMeta{
								Name: "exampleClusterRoleNameTwo",
							},
						},
					},
				}
			})
			It("Should fail when cannot update status condition when all clusterolebindings created successfully", func() {
				gomock.InOrder(
					mockClient.EXPECT().Get(gomock.Any(), testconst.TestNamespaceName, gomock.Any()).Times(1).SetArg(2, testSubjectPermission),
					mockClient.EXPECT().List(gomock.Any(), gomock.Any()).Times(1).SetArg(1, testClusterRoleList),
					mockClient.EXPECT().List(gomock.Any(), gomock.Any()).Times(1).SetArg(1, testClusterRoleBindingList),
					mockClient.EXPECT().Create(gomock.Any(), gomock.Any()).Times(2).SetArg(1, testconst.TestClusterRoleBinding),
					mockClient.EXPECT().Status().Return(mockStatusWriter),
					mockStatusWriter.EXPECT().Update(gomock.Any(), gomock.Any()).SetArg(1, testSubjectPermission).Return(fmt.Errorf("fake error")),
				)
				_, err := subjectPermissionReconciler.Reconcile(testconst.Context, reconcile.Request{NamespacedName: testconst.TestNamespaceName})
				Expect(err).To(HaveOccurred())
			})
		})

		When("Not able to List the NamespaceList successfully", func() {
			BeforeEach(func() {
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
			})
			It("Should report failure", func() {
				gomock.InOrder(
					mockClient.EXPECT().Get(gomock.Any(), testconst.TestNamespaceName, gomock.Any()).Times(1).SetArg(2, testSubjectPermission),
					mockClient.EXPECT().List(gomock.Any(), gomock.Any()).Times(1).SetArg(1, testClusterRoleList),
					mockClient.EXPECT().List(gomock.Any(), gomock.Any()).Times(1).SetArg(1, testClusterRoleBindingList),
					mockClient.EXPECT().List(gomock.Any(), gomock.Any()).Times(1).SetArg(1, *testNamespaceList).Return(fmt.Errorf("fake error")),
				)
				_, err := subjectPermissionReconciler.Reconcile(testconst.Context, reconcile.Request{NamespacedName: testconst.TestNamespaceName})
				Expect(err).To(HaveOccurred())
			})
		})

		When("Not able to update status for a ClusterRoleName not existing as a Role", func() {
			BeforeEach(func() {
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
								NamespacesAllowedRegex: testconst.TestDefaultAllowedList,
								NamespacesDeniedRegex:  testconst.TestEmptyDeniedList,
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
			})
			It("Should report failure", func() {
				gomock.InOrder(
					mockClient.EXPECT().Get(gomock.Any(), testconst.TestNamespaceName, gomock.Any()).Times(1).SetArg(2, testSubjectPermission),
					mockClient.EXPECT().List(gomock.Any(), gomock.Any()).Times(1).SetArg(1, testClusterRoleList),
					mockClient.EXPECT().List(gomock.Any(), gomock.Any()).Times(1).SetArg(1, testClusterRoleBindingList),
					mockClient.EXPECT().List(gomock.Any(), gomock.Any()).Times(1).SetArg(1, *testNamespaceList),
					mockClient.EXPECT().Status().Return(mockStatusWriter),
					mockStatusWriter.EXPECT().Update(gomock.Any(), gomock.Any()).SetArg(1, testSubjectPermission).Return(fmt.Errorf("fake error")),
				)
				_, err := subjectPermissionReconciler.Reconcile(testconst.Context, reconcile.Request{NamespacedName: testconst.TestNamespaceName})
				Expect(err).To(HaveOccurred())
			})
		})

		When("Not able to create RoleBindings in allowed namespaces", func() {
			BeforeEach(func() {
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
								NamespacesAllowedRegex: testconst.TestDefaultAllowedList,
								NamespacesDeniedRegex:  testconst.TestEmptyDeniedList,
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
			})
			It("Should report failure", func() {
				gomock.InOrder(
					mockClient.EXPECT().Get(gomock.Any(), testconst.TestNamespaceName, gomock.Any()).Times(1).SetArg(2, testSubjectPermission),
					mockClient.EXPECT().List(gomock.Any(), gomock.Any()).Times(1).SetArg(1, testClusterRoleList),
					mockClient.EXPECT().List(gomock.Any(), gomock.Any()).Times(1).SetArg(1, testClusterRoleBindingList),
					mockClient.EXPECT().List(gomock.Any(), gomock.Any()).Times(1).SetArg(1, *testNamespaceList),
					mockClient.EXPECT().List(gomock.Any(), gomock.Any(), []client.ListOption{
						client.InNamespace("default"),
					}),
					mockClient.EXPECT().Create(gomock.Any(), gomock.Any()).SetArg(1, *testconst.TestRoleBinding).Return(fmt.Errorf("fake error")),
				)
				_, err := subjectPermissionReconciler.Reconcile(testconst.Context, reconcile.Request{NamespacedName: testconst.TestNamespaceName})
				Expect(err).To(HaveOccurred())
			})
		})

		When("Not able to update status for successful RoleBindings creation in allowed namespaces", func() {
			BeforeEach(func() {
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
								NamespacesAllowedRegex: testconst.TestDefaultAllowedList,
								NamespacesDeniedRegex:  testconst.TestEmptyDeniedList,
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

			})
			It("Should report failure", func() {
				gomock.InOrder(
					mockClient.EXPECT().Get(gomock.Any(), testconst.TestNamespaceName, gomock.Any()).Times(1).SetArg(2, testSubjectPermission),
					mockClient.EXPECT().List(gomock.Any(), gomock.Any()).Times(1).SetArg(1, testClusterRoleList),
					mockClient.EXPECT().List(gomock.Any(), gomock.Any()).Times(1).SetArg(1, testClusterRoleBindingList),
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
			})
		})
	})

	Context("Testing NewClusterRoleBinding function", func() {
		When("A ClusterRoleName, SubjectName and SubjectKind are given", func() {
			It("Should return a ClusterRoleBinding", func() {
				crb := subjectpermission.NewClusterRoleBinding(testClusterRoleName, testSubjectName, testSubjectKind)
				Expect(crb.Name).To(Equal(testClusterRoleName + "-" + testSubjectName))
				Expect(crb.Subjects[0].Kind).To(Equal(testSubjectKind))
				Expect(crb.Subjects[0].Name).To(Equal(testSubjectName))
				Expect(crb.RoleRef.Kind).To(Equal("ClusterRole"))
				Expect(crb.RoleRef.Name).To(Equal(testClusterRoleName))
			})
		})
	})

	Context("Testing PopulateCrClusterRoleNames function", func() {
		When("No ClusterRoleName exists as a ClusterRole", func() {
			It("Should return the entire ClusterRoleName list", func() {
				result := subjectpermission.PopulateCrClusterRoleNames(&testSubjectPermission, &testClusterRoleList)
				Expect(result).To(HaveLen(2))
				Expect(result).To(ContainElement(ContainSubstring("exampleClusterRoleName")))
				Expect(result).To(ContainElement(ContainSubstring("exampleClusterRoleNameTwo")))
			})
		})

		When("One of the ClusterRoleName exists as a ClusterRole", func() {
			BeforeEach(func() {
				testClusterRoleList = rbacv1.ClusterRoleList{
					Items: []rbacv1.ClusterRole{
						{
							ObjectMeta: metav1.ObjectMeta{
								Name: "exampleClusterRoleName",
							},
						},
					},
				}
			})
			It("Should return the remaining ClusterRoleName list", func() {
				result := subjectpermission.PopulateCrClusterRoleNames(&testSubjectPermission, &testClusterRoleList)
				Expect(result).To(HaveLen(1))
				Expect(result).To(ContainElement(ContainSubstring("exampleClusterRoleNameTwo")))
			})
		})

		When("All ClusterRoleName exists as a ClusterRole", func() {
			BeforeEach(func() {
				testClusterRoleList = rbacv1.ClusterRoleList{
					Items: []rbacv1.ClusterRole{
						{
							ObjectMeta: metav1.ObjectMeta{
								Name: "exampleClusterRoleName",
							},
						},
						{
							ObjectMeta: metav1.ObjectMeta{
								Name: "exampleClusterRoleNameTwo",
							},
						},
					},
				}
			})
			It("Should not return any ClusterRoleName list", func() {
				result := subjectpermission.PopulateCrClusterRoleNames(&testSubjectPermission, &testClusterRoleList)
				Expect(result).To(HaveLen(0))
			})
		})
	})

	AfterEach(func() {
		mockCtrl.Finish()
	})
})
