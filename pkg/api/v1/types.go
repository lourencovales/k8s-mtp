package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +kubebuilder:object:root=true
// +kubebuilder:resource:path=tenants,scope=Namespaced,shortName=tn
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Tier",type=string,JSONPath=".spec.tier"
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=".status.phase"
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=".metadata.creationTimestamp"

type Tenant struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   TenantSpec   `json:"spec,omitempty"`
	Status TenantStatus `json:"status,omitempty"`
}

type TenantSpec struct {
	Name       string `json:"name"`
	Tier       Tier   `json:"tier"`
	OwnerEmail string `json:"ownerEmail"`
}

type Tier string

const (
	TierFree       Tier = "free"
	TierPro        Tier = "pro"
	TierEnterprise Tier = "enterprise"
)

type TenantStatus struct {
	Phase      TenantPhase        `json:"phase,omitempty"`
	Namespace  string             `json:"namespace,omitempty"`
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

type TenantPhase string

const (
	TenantPhasePending  TenantPhase = "Pending"
	TenantPhaseCreating TenantPhase = "Creating"
	TenantPhaseActive   TenantPhase = "Active"
	TenantPhaseFailed   TenantPhase = "Failed"
	TenantPhaseDeleting TenantPhase = "Deleting"
)

// +kubebuilder:object:root=true
type TenantList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Tenant `json:"items"`
}
