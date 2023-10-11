// DO NOT REMOVE TAGS BELOW. IF ANY NEW TEST FILES ARE CREATED UNDER /osde2e, PLEASE ADD THESE TAGS TO THEM IN ORDER TO BE EXCLUDED FROM UNIT TESTS.
//go:build osde2e
// +build osde2e

package osde2etests

import (
	"context"

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
		Expect(&clusterRoles).Should(ContainItemWithPrefix(clusterRolePrefix), "unable to find clusterrolebinding with prefix %s", clusterRolePrefix)

		ginkgo.By("cluster role bindings exist")
		var clusterRoleBindings rbacv1.ClusterRoleBindingList
		err = client.List(ctx, &clusterRoleBindings)
		Expect(err).ShouldNot(HaveOccurred(), "unable to list clusterrolebindings")
		Expect(&clusterRoleBindings).Should(ContainItemWithPrefix(clusterRolePrefix), "unable to find clusterrolebinding with prefix %s", clusterRolePrefix)

		ginkgo.By("checking the deployment is available")
		EventuallyDeployment(ctx, client, deploymentName, namespace).Should(BeAvailable())
	})

	// TODO: ginkgo.It("reconciles subjectpermissions", func(ctx context.Context) { })

	ginkgo.It("can be upgraded", func(ctx context.Context) {
		ginkgo.By("forcing operator upgrade")
		err := client.UpgradeOperator(ctx, config.OperatorName, namespace)
		Expect(err).NotTo(HaveOccurred(), "operator upgrade failed")
	})
})
