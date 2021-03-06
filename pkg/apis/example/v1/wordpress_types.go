package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"github.com/operator-framework/operator-sdk/pkg/status"
)

// WordpressSpec defines the desired state of Wordpress
type WordpressSpec struct {
	// Plaintext root password from CRD to create in Secret
	SqlRootPassword string `json:"sqlRootPassword"`

	// Set to true to retain volumes and don't delete PVCs for the Mysql and Wordpress Deployments
	RetainVolumes   bool `json:"retainVolumes,omitempty"`
}

// WordpressStatus defines the observed state of Wordpress
type WordpressStatus struct {
    // Conditions: latest available observations of an object's state
    Conditions status.Conditions `json:"conditions"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Wordpress is the Schema for the wordpresses API
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=wordpresses,scope=Namespaced
// +kubebuilder:storageversion
type Wordpress struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   WordpressSpec   `json:"spec,omitempty"`
	Status WordpressStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// WordpressList contains a list of Wordpress
type WordpressList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Wordpress `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Wordpress{}, &WordpressList{})
}
