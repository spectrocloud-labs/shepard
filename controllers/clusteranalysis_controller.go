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

package controllers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	apierrs "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/net"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/spectrocloud-labs/shepard/api/v1alpha1"
	"github.com/spectrocloud-labs/shepard/pkg/common"
)

// ClusterAnalysisReconciler reconciles a ClusterAnalysis object
type ClusterAnalysisReconciler struct {
	client.Client
	Scheme        *runtime.Scheme
	K8sGptService string
}

//+kubebuilder:rbac:groups=local-ai.spectrocloud-labs.com,resources=clusteranalyses,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=local-ai.spectrocloud-labs.com,resources=clusteranalyses/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=local-ai.spectrocloud-labs.com,resources=clusteranalyses/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *ClusterAnalysisReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)
	log.V(0).Info("Reconciling ClusterAnalysis...")

	analysis := &v1alpha1.ClusterAnalysis{}
	if err := r.Get(ctx, req.NamespacedName, analysis); err != nil {
		// ignore not-found errors, since they can't be fixed by an immediate requeue
		if apierrs.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		log.Error(err, "failed to fetch ClusterAnalysis")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if analysis.Status.Status == common.ClusterAnalysisCompleted || analysis.Status.Status == common.ClusterAnalysisFailed {
		return ctrl.Result{}, nil
	}

	if analysis.Status.Status == "" {
		log.Info("Creating new ClusterAnalysis", "ClusterAnalysis", analysis, "version", analysis.ResourceVersion)

		t := time.Now()
		results := make([]v1alpha1.K8sGptResult, 0)

		analysis.Status.Results = results
		analysis.Status.StartTime = t.Format("01022006-150405")

		if err := r.triggerAnalysis(ctx, analysis, req.NamespacedName); err != nil {
			log.Error(err, "failed to trigger ClusterAnalysis")
			return ctrl.Result{}, nil
		}
	}

	if analysis.Status.Status == "" || analysis.Status.Status == common.ClusterAnalysisInProgress {
		if analysis.Status.Status == "" {
			analysis.Status.Status = common.ClusterAnalysisInProgress
			if err := r.updateAnalysis(ctx, analysis); err != nil {
				return ctrl.Result{}, err
			}
		}
		return ctrl.Result{RequeueAfter: time.Second * 30}, nil
	} else {
		setEndTime(analysis)
		analysis.Status.Status = common.ClusterAnalysisFailed

		if err := r.updateAnalysis(ctx, analysis); err != nil {
			return ctrl.Result{}, err
		}

		log.V(0).Info("Reconcile ClusterAnalysis complete")
		return ctrl.Result{}, nil
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *ClusterAnalysisReconciler) SetupWithManager(mgr ctrl.Manager, options controller.Options) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.ClusterAnalysis{}).
		Complete(r)
}

type K8sGptResponse struct {
	Status   string                  `json:"status"`
	Problems int                     `json:"problems"`
	Results  []v1alpha1.K8sGptResult `json:"results"`
}

func (r *ClusterAnalysisReconciler) triggerAnalysis(ctx context.Context, analysis *v1alpha1.ClusterAnalysis, key types.NamespacedName) error {
	log := log.FromContext(ctx)

	client := &http.Client{}
	url := net.FormatURL("http", r.K8sGptService, 8080, "/analyze")
	req, err := http.NewRequest(http.MethodPost, url.String(), &bytes.Buffer{})
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Connection", "keep-alive")

	q := req.URL.Query()
	q.Add("explain", "true")
	q.Add("namespace", analysis.Spec.Namespace)
	q.Add("nocache", fmt.Sprintf("%t", analysis.Spec.NoCache))
	req.URL.RawQuery = q.Encode()

	go func() {
		log.Info("Generating k8sgpt analysis", "url", req.URL.String())
		resp, err := client.Do(req)
		if err != nil {
			r.failAnalysis(ctx, analysis, err)
		}
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			r.failAnalysis(ctx, analysis, err)
		}
		k8sGptResponse := &K8sGptResponse{}
		if err := json.Unmarshal(body, k8sGptResponse); err != nil {
			r.failAnalysis(ctx, analysis, err)
		}

		// refresh the ClusterAnalysis
		if err := r.Get(ctx, key, analysis); err != nil {
			panic(err)
		}

		setEndTime(analysis)
		analysis.Status.Status = common.ClusterAnalysisCompleted
		analysis.Status.PredictionStatus = k8sGptResponse.Status
		analysis.Status.Problems = k8sGptResponse.Problems
		analysis.Status.Results = k8sGptResponse.Results

		if err := r.updateAnalysis(ctx, analysis); err != nil {
			r.failAnalysis(ctx, analysis, err)
		}
		log.V(0).Info("Updated ClusterAnalysis", "status", analysis.Status.Status)
		log.V(0).Info("Reconcile ClusterAnalysis complete")
	}()

	return nil
}

func (r *ClusterAnalysisReconciler) failAnalysis(ctx context.Context, analysis *v1alpha1.ClusterAnalysis, err error) {
	analysis.Status.Status = common.ClusterAnalysisFailed
	analysis.Status.Results = []v1alpha1.K8sGptResult{
		{
			Error: []v1alpha1.Failure{
				{
					Text: err.Error(),
				},
			},
		},
	}
	setEndTime(analysis)
	if err := r.Status().Update(ctx, analysis); err != nil {
		panic(err)
	}
}

func setEndTime(analysis *v1alpha1.ClusterAnalysis) {
	t := time.Now()
	analysis.Status.EndTime = t.Format("01022006-150405")
}

func (r *ClusterAnalysisReconciler) updateAnalysis(ctx context.Context, analysis *v1alpha1.ClusterAnalysis) error {
	log := log.FromContext(ctx)
	log.Info("Updating ClusterAnalysis", "Status", analysis.Status.Status)

	if err := r.Status().Update(ctx, analysis); err != nil {
		log.Error(err, "failed to update ClusterAnalysis status")
		return err
	}
	return nil
}
