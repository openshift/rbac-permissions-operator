// DO NOT REMOVE TAGS BELOW. IF ANY NEW TEST FILES ARE CREATED UNDER /osde2e, PLEASE ADD THESE TAGS TO THEM IN ORDER TO BE EXCLUDED FROM UNIT TESTS.
//go:build osde2e
// +build osde2e

package osde2etests

import (
	"context"
	"fmt"
	"strings"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	apierrors "k8s.io/apimachinery/pkg/api/errors"

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

		ginkgo.By("checking the clusterrole exists")
		err = client.Get(ctx, deploymentName+"-cluster-admin", "", &rbacv1.ClusterRole{})
		Expect(err).ShouldNot(HaveOccurred(), "clusterrole %s-cluster-admin not found", deploymentName)

		ginkgo.By("checking the clusterrolebinding exists")
		err = client.Get(ctx, deploymentName, "", &rbacv1.ClusterRoleBinding{})
		Expect(err).ShouldNot(HaveOccurred(), "clusterrolebinding %s not found", deploymentName)

		ginkgo.By("checking the deployment is available")
		EventuallyDeployment(ctx, client, deploymentName, namespace).Should(BeAvailable())
	})

	ginkgo.It("reconciles subjectpermissions", func(ctx context.Context) {
		spName := "dedicated-admins"
		testNamespaceName := "test-subjectpermissions"
		ginkgo.By("Working in test namespace " + testNamespaceName)
		// Clean up any leftover namespace from a previous run
		existing := &corev1.Namespace{}
		if err := client.Get(ctx, testNamespaceName, "", existing); err != nil {
			if !apierrors.IsNotFound(err) {
				ginkgo.Fail(fmt.Sprintf("Failed to check for existing test namespace: %v", err))
			}
			// namespace doesn't exist, nothing to clean up
		} else {
			// namespace exists, clean it up
			ginkgo.By("Cleaning up leftover test namespace " + testNamespaceName)
			Expect(client.Delete(ctx, existing)).Should(Succeed(), "Failed to delete leftover test namespace")
			Eventually(func(g Gomega) {
				err := client.Get(ctx, testNamespaceName, "", &corev1.Namespace{})
				g.Expect(apierrors.IsNotFound(err)).To(BeTrue(), fmt.Sprintf("unexpected error: %v", err))
			}).WithTimeout(2 * time.Minute).WithPolling(5 * time.Second).Should(Succeed())
		}

		// Create SubjectPermission test fixture
		ginkgo.By("Creating SubjectPermission test fixture " + spName)
		existingSP := &managedv1alpha1.SubjectPermission{}
		if err := client.WithNamespace(namespace).Get(ctx, spName, namespace, existingSP); err != nil {
			if !apierrors.IsNotFound(err) {
				ginkgo.Fail(fmt.Sprintf("Failed to check for existing SubjectPermission: %v", err))
			}
		} else {
			ginkgo.By("Cleaning up leftover SubjectPermission " + spName)
			Expect(client.Delete(ctx, existingSP)).Should(Succeed(), "Failed to delete leftover SubjectPermission")
			Eventually(func(g Gomega) {
				err := client.WithNamespace(namespace).Get(ctx, spName, namespace, &managedv1alpha1.SubjectPermission{})
				g.Expect(apierrors.IsNotFound(err)).To(BeTrue(), fmt.Sprintf("unexpected error: %v", err))
			}).WithTimeout(30 * time.Second).WithPolling(2 * time.Second).Should(Succeed())
		}

		sp := &managedv1alpha1.SubjectPermission{
			ObjectMeta: metav1.ObjectMeta{
				Name:      spName,
				Namespace: namespace,
			},
			Spec: managedv1alpha1.SubjectPermissionSpec{
				SubjectKind: "Group",
				SubjectName: "dedicated-admins",
				ClusterPermissions: []string{
					"view",
				},
				Permissions: []managedv1alpha1.Permission{
					{
						ClusterRoleName:        "edit",
						NamespacesAllowedRegex: "^test-subjectpermissions$",
					},
				},
			},
		}
		ginkgo.By("Verifying operator deployment is ready and stable")
		EventuallyDeployment(ctx, client, deploymentName, namespace).Should(BeAvailable())
		// Allow controller-runtime informer to complete initial list/watch sync.
		// The CI image deployment may have experienced ImagePullBackOff, leaving
		// the operator pod only seconds old at this point.
		time.Sleep(15 * time.Second)

		Expect(client.Create(ctx, sp)).Should(Succeed(), "Failed to create SubjectPermission test fixture")

		ginkgo.DeferCleanup(func(ctx context.Context) {
			ginkgo.By("Deleting SubjectPermission test fixture " + spName)
			Expect(client.Delete(ctx, sp)).Should(Succeed(), "Failed to delete SubjectPermission test fixture")
		})

		// Wait for the operator to reconcile the SubjectPermission.
		// After a delete+recreate cycle the operator's informer cache may take
		// time to sync the new object, especially on freshly installed PKO
		// ClusterPackage instances where the controller is still starting up.
		ginkgo.By("Waiting for SubjectPermission to be reconciled")
		Eventually(func(g Gomega) {
			var reconciled managedv1alpha1.SubjectPermission
			err := client.WithNamespace(namespace).Get(ctx, spName, namespace, &reconciled)
			g.Expect(err).ShouldNot(HaveOccurred())
			if len(reconciled.Status.Conditions) == 0 {
				// Diagnostic: log operator pod status to aid debugging timeouts.
				var pods corev1.PodList
				if listErr := client.WithNamespace(namespace).List(ctx, &pods); listErr == nil {
					for i := range pods.Items {
						pod := &pods.Items[i]
						if !strings.HasPrefix(pod.Name, deploymentName) {
							continue
						}
						ready := false
						var restarts int32
						for _, cs := range pod.Status.ContainerStatuses {
							if cs.Ready {
								ready = true
							}
							restarts += cs.RestartCount
						}
						age := time.Since(pod.CreationTimestamp.Time).Truncate(time.Second)
						fmt.Fprintf(ginkgo.GinkgoWriter, "  pod %s: phase=%s ready=%t restarts=%d age=%s\n",
							pod.Name, pod.Status.Phase, ready, restarts, age)
					}
				}
			}
			g.Expect(reconciled.Status.Conditions).NotTo(BeEmpty(), "SubjectPermission has no status conditions yet")
		}).WithTimeout(5 * time.Minute).WithPolling(10 * time.Second).Should(Succeed())

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
