package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// KeystoneSpec defines the desired state of Keystone
// +k8s:openapi-gen=true
type KeystoneSpec struct {
	CommonConfiguration  PodConfiguration      `json:"commonConfiguration,omitempty"`
	ServiceConfiguration KeystoneConfiguration `json:"serviceConfiguration"`
}

// KeystoneConfiguration is the Spec for the keystone API.
// +k8s:openapi-gen=true
type KeystoneConfiguration struct {
	MemcachedInstance  string       `json:"memcachedInstance,omitempty"`
	ListenPort         int          `json:"listenPort,omitempty"`
	PostgresInstance   string       `json:"postgresInstance,omitempty"`
	Containers         []*Container `json:"containers,omitempty"`
	KeystoneSecretName string       `json:"keystoneSecretName,omitempty"`
	Region             string       `json:"region,omitempty"`
	// +kubebuilder:validation:Enum=http;https
	AuthProtocol      string `json:"authProtocol,omitempty"`
	UserDomainID      string `json:"userDomainID,omitempty"`
	ProjectDomainID   string `json:"projectDomainID,omitempty"`
	UserDomainName    string `json:"userDomainName,omitempty"`
	ProjectDomainName string `json:"projectDomainName,omitempty"`
	// IP address or domain name (withouth protocol prefix) of the external keystone.
	// If defined no keystone releated resource will be created in cluster and other
	// components will be configured to use this address as keystone endpoint.
	ExternalAddress string `json:"externalAddress,omitempty"`
}

// KeystoneStatus defines the observed state of Keystone
// +k8s:openapi-gen=true
type KeystoneStatus struct {
	Active bool `json:"active,omitempty"`
	Port   int  `json:"port,omitempty"`
	// When keystone is a part of the cluster
	// Endpoint will be set to the service cluster IP.
	// When keystone is external then value of
	// ExternalAddress will be used.
	Endpoint string `json:"endpoint,omitempty"`
	// Set to true when keystone service is not
	// directly managed by controller.
	External bool `json:"external,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Keystone is the Schema for the keystones API
// +k8s:openapi-gen=true
// +kubebuilder:subresource:status
type Keystone struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   KeystoneSpec   `json:"spec,omitempty"`
	Status KeystoneStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// KeystoneList contains a list of Keystone
type KeystoneList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Keystone `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Keystone{}, &KeystoneList{})
}
