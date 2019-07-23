package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// GroupPermissionSpec defines the desired state of GroupPermission
// +k8s:openapi-gen=true
type GroupPermissionSpec struct {
	// Name of the Group granted permissions by the operator
	GroupName string `json:"groupName"`
	// List of permissions applied at Cluster scope
	// +optional
	ClusterPermissions []string `json:"clusterPermissions,omitempty"`
	// List of permissions applied at Namespace scope
	// +optional
	Permissions []Permission `json:"permissions,omitempty"`
}

// Permission deines a Role that is bound to the Group
// Allowed in specific Namespaces
type Permission struct {
	// ClusterRoleName to bind to the Group as a RoleBindings in allowed Namespaces
	ClusterRoleName string `json:"clusterRoleName"`
	// NamespacesAllowedRegex representing allowed Namespaces
	NamespacesAllowedRegex string `json:"namespacesAllowedRegex,omitempty"`
	// NamespacesDeniedRegex representing denied Namespaces
	NamespacesDeniedRegex string `json:"namespacesDeniedRegex,omitempty"`
	// Flag to indicate if "allow" regex is applied first
	// If 'true' order is Allow then Deny, Else order is Deny then Allow
	AllowFirst bool `json:"allowFirst"`
}

// GroupPermissionStatus defines the observed state of GroupPermission
// +k8s:openapi-gen=true
type GroupPermissionStatus struct {
	// List of conditions for the CR
	Conditions []Condition `json:"conditions,omitempty"`
	// State that this condition represents
	State string `json:"state"`
}

// Condition defines a single condition of running the operator against an instance of the GroupPermission CR
type Condition struct {
	// LastTransitionTime is the last time this condition was active for the CR
	LastTransitionTime metav1.Time `json:"lastTransitionTime"`
	// Message related to the condition
	// +optional
	Message string `json:"message,omitempty"`
	// ClusterRoleName in which this condition is true
	ClusterRoleName string `json:"clusterRoleName"`
	// Flag to indicate if condition status is currently active
	Status bool `json:"status"`
	// State that this condition represents
	State GroupPermissionState `json:"state"`
}

// GroupPermissionState defines various states a GroupPermission CR can be in
type GroupPermissionState string

const (
	// GroupPermissionCreated const for Created status
	GroupPermissionCreated GroupPermissionState = "Created"
	// GroupPermissionFailed const for Failed status
	GroupPermissionFailed GroupPermissionState = "Failed"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// GroupPermission is the Schema for the grouppermissions API
// +k8s:openapi-gen=true
// +kubebuilder:subresource:status
type GroupPermission struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   GroupPermissionSpec   `json:"spec,omitempty"`
	Status GroupPermissionStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// GroupPermissionList contains a list of GroupPermission
type GroupPermissionList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []GroupPermission `json:"items"`
}

func init() {
	SchemeBuilder.Register(&GroupPermission{}, &GroupPermissionList{})
}
