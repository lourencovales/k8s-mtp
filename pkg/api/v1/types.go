package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
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

func init() {
	SchemeBuilder.Register(&Tenant{}, &TenantList{})
}

func (in *Tenant) DeepCopyInto(out *Tenant) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	out.Spec = in.Spec
	in.Status.DeepCopyInto(&out.Status)
}

func (in *Tenant) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

func (in *Tenant) DeepCopy() *Tenant {
	if in == nil {
		return nil
	}
	out := &Tenant{}
	in.DeepCopyInto(out)
	return out
}

func (in *TenantList) DeepCopyInto(out *TenantList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]Tenant, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

func (in *TenantList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

func (in *TenantList) DeepCopy() *TenantList {
	if in == nil {
		return nil
	}
	out := &TenantList{}
	in.DeepCopyInto(out)
	return out
}

func (in *TenantStatus) DeepCopyInto(out *TenantStatus) {
	*out = *in
	if in.Conditions != nil {
		in, out := &in.Conditions, &out.Conditions
		*out = make([]metav1.Condition, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

func (in *TenantStatus) DeepCopy() *TenantStatus {
	if in == nil {
		return nil
	}
	out := &TenantStatus{}
	in.DeepCopyInto(out)
	return out
}
