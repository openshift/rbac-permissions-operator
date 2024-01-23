// DO NOT REMOVE TAGS BELOW. IF ANY NEW TEST FILES ARE CREATED UNDER /osde2e, PLEASE ADD THESE TAGS TO THEM IN ORDER TO BE EXCLUDED FROM UNIT TESTS.
//go:build osde2e
// +build osde2e

package osde2etests

import (
	"context"
	"strings"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/openshift/osde2e-common/pkg/clients/openshift"
	. "github.com/openshift/osde2e-common/pkg/gomega/assertions"
	. "github.com/openshift/osde2e-common/pkg/gomega/matchers"
	managedv1alpha1 "github.com/openshift/rbac-permissions-operator/api/v1alpha1"
	"github.com/openshift/rbac-permissions-operator/config"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

var _ = ginkgo.Describe("rbac-permissions-operator", ginkgo.Ordered, func() {
	var (
		client                *openshift.Client
		namespace             = config.OperatorNamespace
		deploymentName        = config.OperatorName
		configMapLockfileName = deploymentName + "-lock"
		rolePrefix            = deploymentName
		clusterRolePrefix     = deploymentName
	)
	ginkgo.BeforeAll(func() {
		log.SetLogger(ginkgo.GinkgoLogr)
		var err error
		client, err = openshift.New(ginkgo.GinkgoLogr)
		Expect(err).ShouldNot(HaveOccurred(), "resources.New error")
		err = managedv1alpha1.AddToScheme(client.GetScheme())
		Expect(err).ShouldNot(HaveOccurred(), "unable to register scheme")
	})

	ginkgo.It("is installed", func(ctx context.Context) {
		ginkgo.By("checking the namespace exists")
		err := client.Get(ctx, namespace, "", &corev1.Namespace{})
		Expect(err).ShouldNot(HaveOccurred(), "namespace %s not found", namespace)

		ginkgo.By("checking the configmap lockfile exists")
		err = client.Get(ctx, configMapLockfileName, namespace, &corev1.ConfigMap{})
		Expect(err).ShouldNot(HaveOccurred(), "configmap lockfile %s not found", configMapLockfileName)

		ginkgo.By("checking the role exists")
		var roles rbacv1.RoleList
		err = client.WithNamespace(namespace).List(ctx, &roles)
		Expect(err).ShouldNot(HaveOccurred(), "failed to list roles")
		Expect(&roles).Should(ContainItemWithPrefix(rolePrefix), "unable to find roles with prefix %s", rolePrefix)

		ginkgo.By("checking the rolebinding exists")
		var rolebindings rbacv1.RoleBindingList
		err = client.List(ctx, &rolebindings)
		Expect(err).ShouldNot(HaveOccurred(), "failed to list rolebindings")
		Expect(&rolebindings).Should(ContainItemWithPrefix(rolePrefix), "unable to find rolebindings with prefix %s", rolePrefix)

		ginkgo.By("checking the clusterroles exists")
		var clusterRoles rbacv1.ClusterRoleList
		err = client.List(ctx, &clusterRoles)
		Expect(err).ShouldNot(HaveOccurred(), "failed to list clusterroles")
		Expect(&clusterRoles).Should(ContainItemWithOLMOwnerWithPrefix(clusterRolePrefix), "unable to find clusterrole with olm owner with prefix %s", clusterRolePrefix)

		ginkgo.By("cluster role bindings exist")
		var clusterRoleBindings rbacv1.ClusterRoleBindingList
		err = client.List(ctx, &clusterRoleBindings)
		Expect(err).ShouldNot(HaveOccurred(), "unable to list clusterrolebindings")
		Expect(&clusterRoleBindings).Should(ContainItemWithOLMOwnerWithPrefix(clusterRolePrefix), "unable to find clusterrolebinding with olm owner with prefix %s", clusterRolePrefix)

		ginkgo.By("checking the deployment is available")
		EventuallyDeployment(ctx, client, deploymentName, namespace).Should(BeAvailable())
	})

	ginkgo.It("reconciles subjectpermissions", func(ctx context.Context) {
		spName := "dedicated-admins"
		testNamespaceName := "test-subjectpermissions"
		ginkgo.By("Working in test namespace " + testNamespaceName)
		testNamespace := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: testNamespaceName}}
		err := client.Create(ctx, testNamespace)
		Expect(err).ShouldNot(HaveOccurred(), "Unable to create test namespace")
		clusterRoles, clusterRoleBindings, roleBindings := getSubjectPermissionRBACInfo(ctx, client, namespace, spName)

		ginkgo.DeferCleanup(func(ctx context.Context) {
			ginkgo.By("Deleting test namespace " + testNamespaceName)
			Expect(client.Delete(ctx, testNamespace)).Should(Succeed(), "Failed to test delete namespace")
		})

		var allClusterRoles rbacv1.ClusterRoleList
		err = client.WithNamespace(testNamespaceName).List(ctx, &allClusterRoles)
		Expect(err).ShouldNot(HaveOccurred(), "failed to list clusterroles")
		ginkgo.By("Checking cluterroles in " + testNamespaceName)
		for _, clusterRoleName := range clusterRoles {
			Expect(&allClusterRoles).Should(ContainItemWithPrefix(clusterRoleName), "subjectpermission clusterrole - "+clusterRoleName+" was not found for "+spName)
		}

		var allClusterRoleBindings rbacv1.ClusterRoleBindingList
		err = client.WithNamespace(testNamespaceName).List(ctx, &allClusterRoleBindings)
		Expect(err).ShouldNot(HaveOccurred(), "failed to list clusterrolebindings")
		ginkgo.By("Checking cluterrolebindings in " + testNamespaceName)
		for _, clusterRoleBindingName := range clusterRoleBindings {
			Expect(&allClusterRoleBindings).Should(ContainItemWithPrefix(clusterRoleBindingName), "subjectpermissions clusterrolebinding - "+clusterRoleBindingName+" was not found for "+spName)
		}

		ginkgo.By("Checking rolebindings in " + testNamespaceName)
		for _, roleBindingName := range roleBindings {
			// can not use "ContainItemWithPrefix" matcher as is, because 120 second polling is needed
			// rolebinding is observed to take a bit more time to create especially if the operator has just been upgraded
			Eventually(ctx, func(ctx context.Context) (bool, error) {
				var allRoleBindings rbacv1.RoleBindingList
				err = client.WithNamespace(testNamespaceName).List(ctx, &allRoleBindings)
				for _, nsRoleBinding := range allRoleBindings.Items {
					if strings.HasPrefix(nsRoleBinding.Name, roleBindingName) {
						return true, nil
					}
				}
				return false, err
			}).WithTimeout(120*time.Second).WithPolling(2*time.Second).WithContext(ctx).Should(BeTrue(),
				"subjectpermissions rolebinding - "+roleBindingName+" was not found for "+spName)
		}

	})

	ginkgo.It("can be upgraded", func(ctx context.Context) {
		ginkgo.By("forcing operator upgrade")
		err := client.UpgradeOperator(ctx, config.OperatorName, namespace)
		Expect(err).NotTo(HaveOccurred(), "operator upgrade failed")
	})
})

func getSubjectPermissionRBACInfo(ctx context.Context, client *openshift.Client, namespace string, spName string) ([]string, []string, []string) {
	var us managedv1alpha1.SubjectPermission
	err := client.WithNamespace(namespace).Get(ctx, spName, namespace, &us)
	Expect(err).ShouldNot(HaveOccurred(), "unable to get subjectpermission")

	clusterRoles := us.Spec.ClusterPermissions

	clusterRoleBindings := []string{}
	for _, crName := range clusterRoles {
		clusterRoleBindings = append(clusterRoleBindings, crName+"-"+us.Name)
	}

	roleBindings := []string{}
	for _, perm := range us.Spec.Permissions {
		clusterRoles = append(clusterRoles, perm.ClusterRoleName)
		roleBindings = append(roleBindings, perm.ClusterRoleName+"-"+us.Name)
	}
	return clusterRoles, clusterRoleBindings, roleBindings
}
