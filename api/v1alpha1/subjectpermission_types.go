/*
Copyright 2022.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.
// +k8s:openapi-gen=true
// SubjectPermissionSpec defines the desired state of SubjectPermission
type SubjectPermissionSpec struct {
	// Important: Run "make" to regenerate code after modifying this file
	// Kind of the Subject that is being granted permissions by the operator
	SubjectKind string `json:"subjectKind"`
	// Name of the Subject granted permissions by the operator
	SubjectName string `json:"subjectName"`
	// List of permissions applied at Cluster scope
	// +optional
	ClusterPermissions []string `json:"clusterPermissions,omitempty"`
	// List of permissions applied at Namespace scope
	// +optional
	Permissions []Permission `json:"permissions,omitempty"`
}

// Permission defines a Role that is bound to the Subject
// Allowed in specific Namespaces
type Permission struct {
	// ClusterRoleName to bind to the Subject as a RoleBindings in allowed Namespaces
	ClusterRoleName string `json:"clusterRoleName"`
	// NamespacesAllowedRegex representing allowed Namespaces
	NamespacesAllowedRegex string `json:"namespacesAllowedRegex,omitempty"`
	// NamespacesDeniedRegex representing denied Namespaces
	NamespacesDeniedRegex string `json:"namespacesDeniedRegex,omitempty"`
}

// +k8s:openapi-gen=true
// SubjectPermissionStatus defines the observed state of SubjectPermission
type SubjectPermissionStatus struct {
	// Important: Run "make" to regenerate code after modifying this file
	// List of conditions for the CR
	Conditions []Condition `json:"conditions,omitempty"`
}

// Condition defines a single condition of running the operator against an instance of the SubjectPermission CR
type Condition struct {
	// Type is the type of the condition
	Type SubjectPermissionType `json:"type,omitempty"`
	// LastTransitionTime is the last time this condition was active for the CR
	LastTransitionTime metav1.Time `json:"lastTransitionTime"`
	// Message related to the condition
	// +optional
	Message string `json:"message,omitempty"`
	// ClusterRoleName in which this condition is true
	ClusterRoleNames []string `json:"clusterRoleName,omitempty"`
	// Flag to indicate if condition status is currently active
	Status bool `json:"status"`
	// State that this condition represents
	State SubjectPermissionState `json:"state"`
}

// SubjectPermissionState defines various states a SubjectPermission CR can be in
type SubjectPermissionState string

// SubjectPermissionType defines various type a SubjectPermission CR can be in
type SubjectPermissionType string

const (
	// ClusterRoleBindingCreated const for ClusterRoleBindingCreated status
	ClusterRoleBindingCreated SubjectPermissionType = "ClusterRoleBindingCreated"
	// RoleBindingCreated const for RoleBindingCreated status
	RoleBindingCreated SubjectPermissionType = "RoleBindingCreated"
	// SubjectPermissionStateCreated const for Created state
	SubjectPermissionStateCreated SubjectPermissionState = "Created"
	// SubjectPermissionStateFailed const for Failed state
	SubjectPermissionStateFailed SubjectPermissionState = "Failed"
)

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +k8s:openapi-gen=true

// SubjectPermission is the Schema for the subjectpermissions API
type SubjectPermission struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   SubjectPermissionSpec   `json:"spec,omitempty"`
	Status SubjectPermissionStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// SubjectPermissionList contains a list of SubjectPermission
type SubjectPermissionList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []SubjectPermission `json:"items"`
}

func init() {
	SchemeBuilder.Register(&SubjectPermission{}, &SubjectPermissionList{})
}
