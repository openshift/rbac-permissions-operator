// Copyright 2019 RedHat
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package config

import (
	auth "k8s.io/api/authorization/v1"
)

const (
	OperatorConfigMapName string = "rbac-permissions-operator"
	OperatorName          string = "rbac-permissions-operator"
	OperatorNamespace     string = "openshift-rbac-permissions-operator"
)

var (
	// OperatorPermissions - All of the permissions the operator needs to run and do its job
	// Ref: Role in deploy directory.
	OperatorPermissions []auth.ResourceAttributes = []auth.ResourceAttributes{
		{Resource: "pods", Verb: "list", Version: "v1", Namespace: OperatorNamespace},
		{Resource: "pods", Verb: "*", Version: "v1", Namespace: OperatorNamespace},
		{Resource: "services", Verb: "*", Version: "v1", Namespace: OperatorNamespace},
		{Resource: "endpoints", Verb: "*", Version: "v1", Namespace: OperatorNamespace},
		{Resource: "persistentvolumeclaims", Verb: "*", Version: "v1", Namespace: OperatorNamespace},
		{Resource: "events", Verb: "*", Version: "v1", Namespace: OperatorNamespace},
		{Resource: "configmaps", Verb: "*", Version: "v1", Namespace: OperatorNamespace},
		{Resource: "secrets", Verb: "*", Version: "v1", Namespace: OperatorNamespace},

		{Group: "apps", Resource: "deployments", Verb: "*", Version: "v1", Namespace: OperatorNamespace},
		{Group: "apps", Resource: "daemonsets", Verb: "*", Version: "extensions/v1beta1", Namespace: OperatorNamespace},
		{Group: "apps", Resource: "replicasets", Verb: "*", Version: "extensions/v1beta1", Namespace: OperatorNamespace},
		{Group: "apps", Resource: "statefulsets", Verb: "*", Version: "apps/v1", Namespace: OperatorNamespace},

		{Group: "monitoring.coreos.com", Resource: "servicemonitors", Verb: "get", Version: "v1", Namespace: OperatorNamespace},
		{Group: "monitoring.coreos.com", Resource: "servicemonitors", Verb: "create", Version: "v1", Namespace: OperatorNamespace},

		{Group: "apps", Resource: "deployments", Verb: "update", Subresource: "finalizers", Name: "rbac-permissions-operator", Namespace: OperatorNamespace},

		{Group: "managed.openshift.io", Resource: "*", Verb: "*", Namespace: OperatorNamespace},
		//{Group: "apps", Verb: "update", Resource: "deployments/finalizers", Subresource: "rbac-permissions-operator"},
	}
)
