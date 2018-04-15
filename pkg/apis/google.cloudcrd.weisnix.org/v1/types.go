package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +genclient:noStatus
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// A Google Cloud project
type Project struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec ProjectSpec `json:"spec"`
}

type ProjectSpec struct {
	Name                 string `json:"name"`
	Region               string `json:"region"`
	Zone                 string `json:"zone"`
	ServiceAccount       string `json:"serviceaccount"`
	ServiceAccountSecret string `json:"serviceaccountsecret"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ProjectList is a list of Project resources
type ProjectList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []Project `json:"items"`
}

// +genclient
// +genclient:noStatus
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// A Google Compute Instance
type Instance struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec InstanceSpec `json:"spec"`
}

type InstanceSpec struct {
	Project string `json:"project"`
	Type     string `json:"type"`
	Image    string `json:"image"`
	DiskSize int64    `json:"disksize"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// InstanceList is a list of Instance resources
type InstanceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []Instance `json:"items"`
}
