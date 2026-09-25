package util

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/openshift/rbac-permissions-operator/api/v1alpha1"
	testconst "github.com/openshift/rbac-permissions-operator/pkg/const/test"
)

var _ = Describe("Controller Utils Tests", func() {

	var (
		mockCtrl            *gomock.Controller
		TestDeniedList      string
		TestConditions      []v1alpha1.Condition
		testClusterRoleList *rbacv1.ClusterRoleList
	)

	BeforeEach(func() {
		mockCtrl = gomock.NewController(GinkgoT())
	})
	AfterEach(func() {
		mockCtrl.Finish()
	})

	Context("Running PopulateCrPermissionClusterRoleNames", func() {

		It("Should give list of all ClusterRoleNames from permissions list if the same ClusterRole is not found", func() {
			crname := PopulateCrPermissionClusterRoleNames(&testconst.TestSubjectPermission, &testconst.TestClusterRoleList)
			Expect(crname).To(ContainElement(ContainSubstring("exampleClusterRoleName")))
			Expect(crname).To(ContainElement(ContainSubstring("testClusterRoleName")))
		})

		It("Should give list of any ClusterRoleName from permissions list if the same ClusterRole is not found", func() {
			testClusterRoleList = &rbacv1.ClusterRoleList{
				Items: []rbacv1.ClusterRole{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "exampleClusterRoleName",
						},
					},
				},
			}
			crname := PopulateCrPermissionClusterRoleNames(&testconst.TestSubjectPermission, testClusterRoleList)
			Expect(crname).To(ContainElement(ContainSubstring("testClusterRoleName")))
		})

		It("Should not give clusterrole name if found", func() {
			testClusterRoleList = &rbacv1.ClusterRoleList{
				Items: []rbacv1.ClusterRole{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "testClusterRoleName",
						},
					},
				},
			}
			crname := PopulateCrPermissionClusterRoleNames(&testconst.TestSubjectPermission, testClusterRoleList)
			Expect(crname).To(ContainElement(ContainSubstring("exampleClusterRoleName")))
			Expect(crname).ToNot(ContainElement(ContainSubstring("testClusterRoleName")))
		})
	})

	Context("Running GenerateSafeList", func() {

		It("Should return safe list if the deny list is blank", func() {
			safeList := GenerateSafeList(testconst.TestDefaultAllowedList, testconst.TestEmptyDeniedList, testconst.TestNamespaceList)
			Expect(safeList).To(ContainElement(ContainSubstring("default.whatever")))
		})

		It("Should not return any list if the deny list is same as allow list", func() {
			TestDeniedList = "default"
			safeList := GenerateSafeList(testconst.TestDefaultAllowedList, TestDeniedList, testconst.TestNamespaceList)
			Expect(safeList).To(BeNil())
		})

		It("Should return safe list if allowed and is not in the deny list", func() {
			TestDeniedList = "something"
			safeList := GenerateSafeList(testconst.TestDefaultAllowedList, TestDeniedList, testconst.TestNamespaceList)
			Expect(safeList).To(ContainElement(ContainSubstring("default")))
		})
	})

	Context("Running NamespaceMatchesPermission", func() {

		It("Should match when the namespace is allowed and not denied", func() {
			Expect(NamespaceMatchesPermission("default", testconst.TestDefaultAllowedList, testconst.TestEmptyDeniedList)).To(BeTrue())
		})

		It("Should not match when the namespace is denied", func() {
			Expect(NamespaceMatchesPermission("default", testconst.TestDefaultAllowedList, "default")).To(BeFalse())
		})

		It("Should not match when the namespace does not match the allow regex", func() {
			Expect(NamespaceMatchesPermission("not-allowed-name", "^only-this$", testconst.TestEmptyDeniedList)).To(BeFalse())
		})

		It("Should be equivalent to GenerateSafeList membership for each namespace", func() {
			// For every namespace in the list, matching it directly must agree with
			// whether GenerateSafeList (the whole-list scan) includes it. This proves
			// the Namespace controller's single-name match is behavior-preserving.
			allowed := testconst.TestDefaultAllowedList
			denied := "default"
			safeList := GenerateSafeList(allowed, denied, testconst.TestNamespaceList)
			inSafeList := func(name string) bool {
				for _, n := range safeList {
					if n == name {
						return true
					}
				}
				return false
			}
			for _, nsItem := range testconst.TestNamespaceList.Items {
				Expect(NamespaceMatchesPermission(nsItem.Name, allowed, denied)).To(
					Equal(inSafeList(nsItem.Name)),
					"mismatch for namespace %q", nsItem.Name,
				)
			}
		})
	})

	Context("Running NewRoleBindingForClusterRole", func() {

		It("Should return the expected rolebinding", func() {
			rb := NewRoleBindingForClusterRole("examplePermissionClusterRoleName", "exampleGroupName", "", "Group", "examplenamespace")
			Expect(rb).To(Equal(testconst.TestRoleBinding))
		})

		It("Should return the expected rolebinding for SA", func() {
			rb := NewRoleBindingForClusterRole("examplePermissionClusterRoleName", "exampleGroupName", "exampleGroupNamespace", "ServiceAccount", "examplenamespace")
			Expect(rb).To(Equal(testconst.TestRoleBindingSA))
		})
	})

	Context("Running UpdateCondition", func() {

		It("Updates the conditions as expected by adding the clusterrole with no existing condition", func() {
			conditions := UpdateCondition(testconst.TestConditions, "testMessage", []string{"testClusterRoleName"}, false, testconst.TestSubjectPermissionState, testconst.TestSubjectPermissionType)
			Expect(conditions).To(HaveLen(3))
			Expect(conditions[2].ClusterRoleNames[0]).To(Equal("testClusterRoleName"))
			Expect(conditions[2].Type).To(Equal(testconst.TestSubjectPermissionType))
		})

		It("Updates the conditions as expected by adding the clusterrole with existing condition", func() {
			TestConditions = []v1alpha1.Condition{
				{
					ClusterRoleNames: []string{"exampleClusterRoleName"},
					Message:          "exampleMessage",
					Status:           true,
					State:            "exampleState",
				},
				{
					ClusterRoleNames: []string{"testClusterRoleName"},
					Type:             testconst.TestSubjectPermissionType,
				},
			}
			conditions := UpdateCondition(TestConditions, "testMessage", []string{"testClusterRoleName"}, false, testconst.TestSubjectPermissionState, testconst.TestSubjectPermissionType)
			Expect(conditions).To(HaveLen(2))
			Expect(conditions[1].ClusterRoleNames[0]).To(Equal("testClusterRoleName"))
			Expect(conditions[1].State).To(Equal(testconst.TestSubjectPermissionState))
			Expect(conditions[1].Status).To(Equal(false))
		})

		It("Updates the conditions as expected by adding the clusterrole with existing condition but with different status", func() {
			TestConditions = []v1alpha1.Condition{
				{
					ClusterRoleNames: []string{"exampleClusterRoleName"},
					Message:          "exampleMessage",
					Status:           true,
					State:            "exampleState",
				},
				{
					ClusterRoleNames: []string{"testClusterRoleName"},
					Type:             testconst.TestSubjectPermissionType,
				},
			}
			conditions := UpdateCondition(TestConditions, "testMessage", []string{"testClusterRoleName"}, true, testconst.TestSubjectPermissionState, testconst.TestSubjectPermissionType)
			Expect(conditions).To(HaveLen(2))
			Expect(conditions[1].ClusterRoleNames[0]).To(Equal("testClusterRoleName"))
			Expect(conditions[1].State).To(Equal(testconst.TestSubjectPermissionState))
			Expect(conditions[1].Status).To(Equal(true))
		})

	})

})
