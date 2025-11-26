package namespace_test

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/openshift/rbac-permissions-operator/api/v1alpha1"
	"github.com/openshift/rbac-permissions-operator/controllers/namespace"
	testconst "github.com/openshift/rbac-permissions-operator/pkg/const/test"
	clientmocks "github.com/openshift/rbac-permissions-operator/pkg/util/test/generated/mocks/client"
)

var _ = Describe("Namespace Controller", func() {
	var (
		mockClient                *clientmocks.MockClient
		mockCtrl                  *gomock.Controller
		mockStatusWriter          *clientmocks.MockStatusWriter
		namespaceReconciler       namespace.NamespaceReconciler
		testNamespace             *corev1.Namespace
		testNamespaceList         *corev1.NamespaceList
		testSubjectPermissionList v1alpha1.SubjectPermissionList
		testRoleBinding           *rbacv1.RoleBinding
		testRoleBindingList       *rbacv1.RoleBindingList
		ns                        string
		safeList                  []string
	)

	BeforeEach(func() {
		mockCtrl = gomock.NewController(GinkgoT())
		mockClient = clientmocks.NewMockClient(mockCtrl)
		mockStatusWriter = clientmocks.NewMockStatusWriter(mockCtrl)
		namespaceReconciler = namespace.NamespaceReconciler{
			Client: mockClient,
			Scheme: testconst.Scheme,
		}
		testSubjectPermissionList = *testconst.TestSubjectPermissionList
		testNamespace = &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name:      testconst.TestNamespaceName.Name,
				Namespace: testconst.TestNamespaceName.Namespace,
			},
			Spec:   corev1.NamespaceSpec{},
			Status: corev1.NamespaceStatus{},
		}
		testNamespaceList = testconst.TestNamespaceList
		testRoleBinding = testconst.TestRoleBinding
		testRoleBindingList = testconst.TestRoleBindingList
		ns = testconst.TestNamespaceName.Name
		safeList = []string{"test", "default"}
	})

	Context("Reconciling Namespace", func() {

		When("Namespace is not in the safe list", func() {
			It("Updates the status condition", func() {
				gomock.InOrder(
					mockClient.EXPECT().Get(gomock.Any(), testconst.TestNamespaceName, gomock.Any()).Times(1).SetArg(2, *testNamespace),
					mockClient.EXPECT().List(gomock.Any(), gomock.Any()).Times(1).SetArg(1, *testNamespaceList),
					mockClient.EXPECT().List(gomock.Any(), gomock.Any()).Times(1).SetArg(1, testSubjectPermissionList),
					mockClient.EXPECT().List(gomock.Any(), gomock.Any(), []client.ListOption{
						client.InNamespace(testNamespace.Name),
					}).Times(1).SetArg(1, *testconst.TestRoleBindingList),
					mockClient.EXPECT().Status().Return(mockStatusWriter),
					mockStatusWriter.EXPECT().Update(gomock.Any(), gomock.Any()).Times(1).DoAndReturn(
						func(ctx context.Context, sp *v1alpha1.SubjectPermission, uo ...client.UpdateOption) error {
							Expect(sp.Status.Conditions[1].Message).To(Equal("Successfully created all roleBindings"))
							Expect(sp.Status.Conditions[1].ClusterRoleNames).To(ContainElement(ContainSubstring("exampleClusterRoleName")))
							Expect(sp.Status.Conditions[1].ClusterRoleNames).To(ContainElement(ContainSubstring("testClusterRoleName")))
							Expect(sp.Status.Conditions[1].Status).To(Equal(true))
							Expect(sp.Status.Conditions[1].State).To(Equal(v1alpha1.SubjectPermissionStateCreated))
							Expect(sp.Status.Conditions[1].Type).To(Equal(v1alpha1.RoleBindingCreated))
							return nil
						}),
				)
				_, err := namespaceReconciler.Reconcile(testconst.Context, reconcile.Request{NamespacedName: testconst.TestNamespaceName})
				Expect(err).ToNot(HaveOccurred())
			})
		})

		When("Namespace is in the safe list", func() {
			BeforeEach(func() {
				testNamespaceList = &corev1.NamespaceList{
					Items: []corev1.Namespace{
						{
							ObjectMeta: testNamespace.ObjectMeta,
						},
					},
				}
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
			})
			It("Creates new rolebinding and updates status condition", func() {
				gomock.InOrder(
					mockClient.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).Times(1).SetArg(2, *testNamespace),
					mockClient.EXPECT().List(gomock.Any(), gomock.Any()).Times(1).SetArg(1, *testNamespaceList),
					mockClient.EXPECT().List(gomock.Any(), gomock.Any()).Times(1).SetArg(1, testSubjectPermissionList),
					mockClient.EXPECT().List(gomock.Any(), gomock.Any(), []client.ListOption{
						client.InNamespace(testNamespace.Name),
					}).Times(1).SetArg(1, *testconst.TestRoleBindingList),
					mockClient.EXPECT().Create(gomock.Any(), gomock.Any()).DoAndReturn(
						func(ctx context.Context, rb *rbacv1.RoleBinding, co ...client.CreateOption) error {
							Expect(rb.ObjectMeta.Name).To(Equal(fmt.Sprintf("%s-%s",
								testSubjectPermissionList.Items[0].Spec.Permissions[0].ClusterRoleName,
								testSubjectPermissionList.Items[0].Spec.SubjectName)))
							Expect(rb.ObjectMeta.Namespace).To(Equal(testNamespace.Name))
							Expect(rb.Subjects[0].Kind).To(Equal(testSubjectPermissionList.Items[0].Spec.SubjectKind))
							Expect(rb.Subjects[0].Name).To(Equal(testSubjectPermissionList.Items[0].Spec.SubjectName))
							Expect(rb.RoleRef.Kind).To(Equal("ClusterRole"))
							Expect(rb.RoleRef.Name).To(Equal(testSubjectPermissionList.Items[0].Spec.Permissions[0].ClusterRoleName))
							return nil
						}),
					mockClient.EXPECT().Status().Return(mockStatusWriter),
					mockStatusWriter.EXPECT().Update(gomock.Any(), gomock.Any()).Times(1).DoAndReturn(
						func(ctx context.Context, sp *v1alpha1.SubjectPermission, uo ...client.UpdateOption) error {
							Expect(sp.Status.Conditions[1].Message).To(Equal("Successfully created all roleBindings"))
							Expect(sp.Status.Conditions[1].ClusterRoleNames).To(ContainElement(ContainSubstring("testClusterRoleName")))
							Expect(sp.Status.Conditions[1].ClusterRoleNames).ToNot(ContainElement(ContainSubstring("exampleClusterRoleName")))
							Expect(sp.Status.Conditions[1].Status).To(Equal(true))
							Expect(sp.Status.Conditions[1].State).To(Equal(v1alpha1.SubjectPermissionStateCreated))
							Expect(sp.Status.Conditions[1].Type).To(Equal(v1alpha1.RoleBindingCreated))
							return nil
						}),
				)
				_, err := namespaceReconciler.Reconcile(testconst.Context, reconcile.Request{NamespacedName: testconst.TestNamespaceName})
				Expect(err).ToNot(HaveOccurred())
			})
		})

		When("Not able to Get the namespace instance", func() {
			It("Should report failure", func() {
				gomock.InOrder(
					mockClient.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).Return(fmt.Errorf("fake error")),
				)
				_, err := namespaceReconciler.Reconcile(testconst.Context, reconcile.Request{NamespacedName: testconst.TestNamespaceName})
				Expect(err).Should(HaveOccurred())
			})
		})

		When("Not able to List the NamespaceList", func() {
			It("Should report failure", func() {
				gomock.InOrder(
					mockClient.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).SetArg(2, *testNamespace),
					mockClient.EXPECT().List(gomock.Any(), gomock.Any()).Return(fmt.Errorf("fake error")),
				)
				_, err := namespaceReconciler.Reconcile(testconst.Context, reconcile.Request{NamespacedName: testconst.TestNamespaceName})
				Expect(err).Should(HaveOccurred())
			})
		})

		When("Not able to List the SubjectPermissionList", func() {
			It("Should report failure", func() {
				gomock.InOrder(
					mockClient.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).SetArg(2, *testNamespace),
					mockClient.EXPECT().List(gomock.Any(), gomock.Any()).SetArg(1, *testNamespaceList),
					mockClient.EXPECT().List(gomock.Any(), gomock.Any()).Return(fmt.Errorf("fake error")),
				)
				_, err := namespaceReconciler.Reconcile(testconst.Context, reconcile.Request{NamespacedName: testconst.TestNamespaceName})
				Expect(err).Should(HaveOccurred())
			})
		})

		When("Not able to List the RoleBindingList", func() {
			It("Should report failure", func() {
				gomock.InOrder(
					mockClient.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).SetArg(2, *testNamespace),
					mockClient.EXPECT().List(gomock.Any(), gomock.Any()).SetArg(1, *testNamespaceList),
					mockClient.EXPECT().List(gomock.Any(), gomock.Any()).SetArg(1, testSubjectPermissionList),
					mockClient.EXPECT().List(gomock.Any(), gomock.Any(), gomock.Any()).Return(fmt.Errorf("fake error")),
				)
				_, err := namespaceReconciler.Reconcile(testconst.Context, reconcile.Request{NamespacedName: testconst.TestNamespaceName})
				Expect(err).Should(HaveOccurred())
			})
		})

		When("Not able to Create the RoleBinding", func() {
			BeforeEach(func() {
				testNamespaceList = &corev1.NamespaceList{
					Items: []corev1.Namespace{
						{
							ObjectMeta: testNamespace.ObjectMeta,
						},
					},
				}
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
			})
			It("Should report failure", func() {
				gomock.InOrder(
					mockClient.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).Times(1).SetArg(2, *testNamespace),
					mockClient.EXPECT().List(gomock.Any(), gomock.Any()).Times(1).SetArg(1, *testNamespaceList),
					mockClient.EXPECT().List(gomock.Any(), gomock.Any()).Times(1).SetArg(1, testSubjectPermissionList),
					mockClient.EXPECT().List(gomock.Any(), gomock.Any(), []client.ListOption{
						client.InNamespace(testNamespace.Name),
					}).Times(1).SetArg(1, *testconst.TestRoleBindingList),
					mockClient.EXPECT().Create(gomock.Any(), gomock.Any()).Return(fmt.Errorf("fake error")),
				)
				_, err := namespaceReconciler.Reconcile(testconst.Context, reconcile.Request{NamespacedName: testconst.TestNamespaceName})
				Expect(err).Should(HaveOccurred())
			})
		})

		When("Not able to Update status condition for successful RoleBinding creation", func() {
			BeforeEach(func() {
				testNamespaceList = &corev1.NamespaceList{
					Items: []corev1.Namespace{
						{
							ObjectMeta: testNamespace.ObjectMeta,
						},
					},
				}
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
			})
			It("Should report failure", func() {
				gomock.InOrder(
					mockClient.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).Times(1).SetArg(2, *testNamespace),
					mockClient.EXPECT().List(gomock.Any(), gomock.Any()).Times(1).SetArg(1, *testNamespaceList),
					mockClient.EXPECT().List(gomock.Any(), gomock.Any()).Times(1).SetArg(1, testSubjectPermissionList),
					mockClient.EXPECT().List(gomock.Any(), gomock.Any(), []client.ListOption{
						client.InNamespace(testNamespace.Name),
					}).Times(1).SetArg(1, *testconst.TestRoleBindingList),
					mockClient.EXPECT().Create(gomock.Any(), gomock.Any()).DoAndReturn(
						func(ctx context.Context, rb *rbacv1.RoleBinding, co ...client.CreateOption) error {
							Expect(rb.ObjectMeta.Name).To(Equal(fmt.Sprintf("%s-%s",
								testSubjectPermissionList.Items[0].Spec.Permissions[0].ClusterRoleName,
								testSubjectPermissionList.Items[0].Spec.SubjectName)))
							Expect(rb.ObjectMeta.Namespace).To(Equal(testNamespace.Name))
							Expect(rb.Subjects[0].Kind).To(Equal(testSubjectPermissionList.Items[0].Spec.SubjectKind))
							Expect(rb.Subjects[0].Name).To(Equal(testSubjectPermissionList.Items[0].Spec.SubjectName))
							Expect(rb.RoleRef.Kind).To(Equal("ClusterRole"))
							Expect(rb.RoleRef.Name).To(Equal(testSubjectPermissionList.Items[0].Spec.Permissions[0].ClusterRoleName))
							return nil
						}),
					mockClient.EXPECT().Status().Return(mockStatusWriter),
					mockStatusWriter.EXPECT().Update(gomock.Any(), gomock.Any()).Times(1).Return(fmt.Errorf("fake error")),
				)
				_, err := namespaceReconciler.Reconcile(testconst.Context, reconcile.Request{NamespacedName: testconst.TestNamespaceName})
				Expect(err).Should(HaveOccurred())
			})
		})
	})

	Context("Testing NamespaceInSlice function", func() {
		When("Namespace is in the allowed list", func() {
			It("Should return true", func() {
				result := namespace.NamespaceInSlice(ns, safeList)
				Expect(result).To(BeTrue())
			})
		})

		When("Namespace is not in the allowed list", func() {
			BeforeEach(func() {
				ns = "test"
				safeList = []string{"default"}
			})
			It("Should return false", func() {
				result := namespace.NamespaceInSlice(ns, safeList)
				Expect(result).To(BeFalse())
			})
		})
	})

	Context("Testing RolebindingInNamespace function", func() {
		When("RoleBinding is in the RoleBindingList", func() {
			It("Should return true", func() {
				result := namespace.RolebindingInNamespace(testRoleBinding, testRoleBindingList)
				Expect(result).To(BeTrue())
			})
		})

		When("RoleBinding is not in the RoleBindingList", func() {
			BeforeEach(func() {
				testRoleBindingList = &rbacv1.RoleBindingList{
					Items: []rbacv1.RoleBinding{
						{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "testPermissionCLusterRoleName-testGroupName",
								Namespace: "test-namespace",
							},
						},
					},
				}
			})
			It("Should return false", func() {
				result := namespace.RolebindingInNamespace(testRoleBinding, testRoleBindingList)
				Expect(result).To(BeFalse())
			})
		})
	})

	// Additional edge case test
	When("SubjectPermissionList fails", func() {
		It("Should return error", func() {
			listError := fmt.Errorf("subjectpermission list failed")
			gomock.InOrder(
				mockClient.EXPECT().Get(gomock.Any(), testconst.TestNamespaceName, gomock.Any()).Times(1).SetArg(2, *testNamespace),
				mockClient.EXPECT().List(gomock.Any(), gomock.Any()).Times(1).SetArg(1, *testNamespaceList),
				mockClient.EXPECT().List(gomock.Any(), gomock.Any()).Times(1).Return(listError),
			)
			_, err := namespaceReconciler.Reconcile(testconst.Context, reconcile.Request{NamespacedName: testconst.TestNamespaceName})
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed to list SubjectPermissions"))
		})
	})

	AfterEach(func() {
		mockCtrl.Finish()
	})
})
