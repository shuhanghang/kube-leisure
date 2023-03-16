/*
Copyright 2023.

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

package v1beta1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type RestartSchema struct {
	ResourceType string `json:"resourceType"`
	Name         string `json:"name"`
	NameSpace    string `json:"nameSpace"`
	RestartAt    string `json:"restartAt"`
	TimeZone     string `json:"timeZone"`
}

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// LeisureSpec defines the desired state of Leisure
type LeisureSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Restart Scheme
	Restart RestartSchema `json:"restart,omitempty"`
}

// LeisureStatus defines the observed state of Leisure
type LeisureStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Next time
	NextAt string `json:"nextAt,omitempty"`
}

// +kubebuilder:printcolumn:name="ResourceType",type="string",JSONPath=".spec.restart.resourceType",description=""
// +kubebuilder:printcolumn:name="RestartAt",type="string",JSONPath=".spec.restart.restartAt",description=""
// +kubebuilder:printcolumn:name="TimeZone",type="string",JSONPath=".spec.restart.timeZone",description=""
// +kubebuilder:printcolumn:name="NextAt",type="string",JSONPath=".status.nextAt",description=""
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// Leisure is the Schema for the leisures API
type Leisure struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   LeisureSpec   `json:"spec,omitempty"`
	Status LeisureStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// LeisureList contains a list of Leisure
type LeisureList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Leisure `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Leisure{}, &LeisureList{})
}
