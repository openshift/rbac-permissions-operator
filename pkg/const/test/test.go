package test

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"

	v1alpha1 "github.com/openshift/rbac-permissions-operator/pkg/apis/managed/v1alpha1"
)

var (
	Context = context.TODO()
	Scheme  = setScheme(runtime.NewScheme())

	TestSubjectPermission = v1alpha1.SubjectPermission{
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
					ClusterRoleName: "exampleClusterRoleName",
				},
				{
					ClusterRoleName: "testClusterRoleName",
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

	TestClusterRoleList = rbacv1.ClusterRoleList{
		Items: []rbacv1.ClusterRole{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "exampleClusterRoleName2",
				},
				Rules: []rbacv1.PolicyRule{
					{
						APIGroups: []string{"rbac.authorization.k8s.io"},
						Resources: []string{"clusterrolebindings"},
						Verbs:     []string{"create", "delete", "get", "list"},
					},
				},
			},
		},
	}

	TestNamespaceList = &corev1.NamespaceList{
		Items: []corev1.Namespace{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "openshift.admin-stuff",
				},
			},
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "default.whatever",
				},
			},
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "openshift.readers",
				},
			},
		},
	}

	TestAllowedList = "default"

	TestDeniedList = ""

	TestRoleBinding = &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "examplePermissionClusterRoleName-exampleGroupName",
			Namespace: "examplenamespace",
		},
		Subjects: []rbacv1.Subject{
			{
				Kind: "Group",
				Name: "exampleGroupName",
			},
		},
		RoleRef: rbacv1.RoleRef{
			Kind: "ClusterRole",
			Name: "examplePermissionClusterRoleName",
		},
	}

	TestConditions = []v1alpha1.Condition{
		{
			ClusterRoleNames: []string{"exampleClusterRoleName"},
			Message:          "exampleMessage",
			Status:           true,
			State:            "exampleState",
		},
		{
			ClusterRoleNames: []string{"testClusterRoleName"},
			Message:          "testMessage",
			Status:           false,
			State:            "testState",
		},
	}

	TestSubjectPermissionState v1alpha1.SubjectPermissionState = "testState"

	TestSubjectPermissionType v1alpha1.SubjectPermissionType = "testType"
)

func setScheme(scheme *runtime.Scheme) *runtime.Scheme {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(v1alpha1.SchemeBuilder.AddToScheme(scheme))
	return scheme
}
