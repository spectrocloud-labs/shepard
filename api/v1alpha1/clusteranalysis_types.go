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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ClusterAnalysisSpec defines the desired state of ClusterAnalysis
type ClusterAnalysisSpec struct {
	FilterKinds []string `json:"filterKinds,omitempty"`
	Namespace   string   `json:"namespace,omitempty"`
	NoCache     bool     `json:"noCache,omitempty"`
}

// ClusterAnalysisStatus defines the observed state of ClusterAnalysis
type ClusterAnalysisStatus struct {
	Status           string         `json:"status,omitempty"`
	StartTime        string         `json:"startTime,omitempty"`
	EndTime          string         `json:"endTime,omitempty"`
	PredictionStatus string         `json:"predictionStatus,omitempty"`
	Problems         int            `json:"problems,omitempty"`
	Results          []K8sGptResult `json:"results,omitempty"`
}

type K8sGptResult struct {
	Kind         string    `json:"kind,omitempty"`
	Name         string    `json:"name,omitempty"`
	Error        []Failure `json:"error,omitempty"`
	Details      string    `json:"details,omitempty"`
	ParentObject string    `json:"parentObject,omitempty"`
}

type Failure struct {
	Text      string      `json:"text,omitempty"`
	Sensitive []Sensitive `json:"sensitive,omitempty"`
}

type Sensitive struct {
	Unmasked string `json:"unmasked,omitempty"`
	Masked   string `json:"masked,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// ClusterAnalysis is the Schema for the clusteranalyses API
type ClusterAnalysis struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ClusterAnalysisSpec   `json:"spec,omitempty"`
	Status ClusterAnalysisStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// ClusterAnalysisList contains a list of ClusterAnalysis
type ClusterAnalysisList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ClusterAnalysis `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ClusterAnalysis{}, &ClusterAnalysisList{})
}
