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
	"context"
	"reflect"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/errors"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"github.com/robfig/cron/v3"
	leisurev1beta1 "github.com/shuhanghang/kube-leisure/api/v1beta1"
	"go.uber.org/zap"
)

var MyCrontab = cron.New()

var MyCronCache = map[string]JobSchema{}

// LeisureReconciler reconciles a Leisure object
type LeisureReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	log    *zap.Logger
}

type JobGroup struct {
	Name string
	Req  ctrl.Request
	Job  JobSchema
}

type JobSchema struct {
	Name      string
	NameSpace string
	Crontab   string
	Type      string
	Id        cron.EntryID
	UID       string
}

type CronJobsCache map[string]map[string]int32
type CronJobCache map[string]int32

//+kubebuilder:rbac:groups=leisure.shuhanghang.com,resources=leisures,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=leisure.shuhanghang.com,resources=leisures/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=leisure.shuhanghang.com,resources=leisures/finalizers,verbs=update

func (r *LeisureReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	r.log, _ = zap.NewDevelopment()
	var JobGroup = JobGroup{
		Name: req.NamespacedName.String(),
		Req:  req,
	}
	leisureRestart := leisurev1beta1.Leisure{}

	err := r.Get(context.TODO(), req.NamespacedName, &leisureRestart)
	if err != nil {
		if errors.IsNotFound(err) {
			if !reflect.DeepEqual(MyCronCache[JobGroup.Name], CronJobCache{}) {
				job := MyCronCache[JobGroup.Name]
				if job.Id != 0 {
					MyCrontab.Remove(job.Id)
					r.log.Sugar().Infof("The job has been deleted, JobID: %v, Lens of crontab entries: %v, Job: %#v", job.Id, len(MyCrontab.Entries()), job)
				}

			} else {
				r.log.Info("Not found leisure object, it may be removed")
			}
			return ctrl.Result{}, nil
		}
	}

	Job := JobSchema{
		Name:      leisureRestart.Spec.Restart.Name,
		NameSpace: leisureRestart.Spec.Restart.NameSpace,
		Crontab:   "CRON_TZ=" + leisureRestart.Spec.Restart.TimeZone + " " + leisureRestart.Spec.Restart.RestartAt,
		Type:      leisureRestart.Spec.Restart.ResourceType,
	}

	Job.UID = string(leisureRestart.UID)
	JobGroup.Job = Job

	// Delete job
	if !reflect.DeepEqual(MyCronCache[JobGroup.Name], CronJobCache{}) {
		jobId := MyCronCache[JobGroup.Name].Id
		if jobId != 0 {
			MyCrontab.Remove(jobId)
			r.log.Sugar().Infof("The job has been deleted, JobID: %v, Lens of cron entries: %v, Job: %#v", jobId, len(MyCrontab.Entries()), JobGroup.Job)
		}
		r.log.Sugar().Infof("Restart job has been checked, Job: %v", JobGroup.Name)
	}

	// Add job
	if JobGroup.Job.Type == "deployment" {
		ei, _ := MyCrontab.AddFunc(JobGroup.Job.Crontab, func() { r.AddRolloutRestartDeploy(JobGroup) })
		JobGroup.Job.Id = ei
		MyCronCache[JobGroup.Name] = JobGroup.Job
		r.log.Sugar().Infof("Launch the job and set cronId to cronCache, JobID: %v, Lens of cron entries: %v, Job: %#v", ei, len(MyCrontab.Entries()), JobGroup.Job)

	} else if JobGroup.Job.Type == "statefulset" {
		ei, _ := MyCrontab.AddFunc(JobGroup.Job.Crontab, func() { r.AddRolloutRestartStatefulset(JobGroup) })
		JobGroup.Job.Id = ei
		MyCronCache[JobGroup.Name] = JobGroup.Job
		r.log.Sugar().Infof("Launch the job and set cronId to cronCache, JobID: %v, Lens of cron entries: %v, Job: %#v", ei, len(MyCrontab.Entries()), JobGroup.Job)
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *LeisureReconciler) SetupWithManager(mgr ctrl.Manager) error {
	pred := predicate.GenerationChangedPredicate{}
	return ctrl.NewControllerManagedBy(mgr).
		For(&leisurev1beta1.Leisure{}).
		WithEventFilter(pred).
		Complete(r)
}

func (r *LeisureReconciler) AddRolloutRestartDeploy(restartDep JobGroup) {
	dep := appsv1.Deployment{}
	specDeployment := types.NamespacedName{
		Name:      restartDep.Job.Name,
		Namespace: restartDep.Job.NameSpace,
	}
	err := r.Get(context.TODO(), specDeployment, &dep)
	if err != nil {
		r.log.Sugar().Errorf("error to get the deployment, deployment: %v, error: %v", specDeployment, err.Error())
		return
	}
	if reflect.DeepEqual(dep, appsv1.Deployment{}) {
		r.log.Sugar().Infof("Not found the deployment, JobName: %v, JobNameSpace: %v", specDeployment.Name, specDeployment.Namespace)
		return
	}
	patch := client.MergeFrom(dep.DeepCopy())
	restartAt := time.Now().Format("2006-01-02 15:04:05")
	dep.Spec.Template.Annotations = map[string]string{}
	dep.Spec.Template.Annotations["kubectl.kubernetes.io/restartedAt"] = restartAt
	err = r.Patch(context.TODO(), &dep, patch)
	if err != nil {
		r.log.Sugar().Errorf("error to restart the Deployment, deployment: %v", specDeployment)
		return
	}
	r.UpdateStatus(restartDep)
	r.log.Sugar().Infof("Deployment has being restarted, RestartAt: %v, NextAt: %v, Job: %v", restartAt, restartDep, specDeployment)
}

func (r *LeisureReconciler) AddRolloutRestartStatefulset(restartState JobGroup) {
	dep := appsv1.StatefulSet{}
	specStatefulset := types.NamespacedName{
		Name:      restartState.Job.Name,
		Namespace: restartState.Job.NameSpace,
	}
	err := r.Get(context.TODO(), specStatefulset, &dep)
	if err != nil {
		r.log.Sugar().Errorf("error to get the StatefulSet, StatefulSet: %v", specStatefulset)
		return
	}
	if reflect.DeepEqual(dep, appsv1.StatefulSet{}) {
		r.log.Sugar().Infof("Not found the statefulset, JobName: %v, JobNameSpace: %v", specStatefulset, specStatefulset.Namespace)
		return
	}
	patch := client.MergeFrom(dep.DeepCopy())
	restartAt := time.Now().Format("2006-01-02 15:04:05")
	dep.Spec.Template.Annotations = map[string]string{}
	dep.Spec.Template.Annotations["kubectl.kubernetes.io/restartedAt"] = restartAt
	err = r.Patch(context.TODO(), &dep, patch)
	if err != nil {
		r.log.Sugar().Errorf("error to restart the StatefuleSet, StatefulSet: %#v", specStatefulset)
		return
	}
	r.UpdateStatus(restartState)
	r.log.Sugar().Infof("StatefulSet has being restartted, RestartAt: %v, NextAt: %v, Job: %#v", restartAt, GetJobNextAt(restartState), specStatefulset)

}

// Update Status
func (r *LeisureReconciler) UpdateStatus(Job JobGroup) {
	leisure := leisurev1beta1.Leisure{}
	err := r.Get(context.TODO(), Job.Req.NamespacedName, &leisure)
	if err == nil {
		leisure.Status.NextAt = GetJobNextAt(Job)
		r.Status().Update(context.TODO(), &leisure)
	}
}

// Get job Next time
func GetJobNextAt(Job JobGroup) string {
	jobId := MyCronCache[Job.Name].Id
	if jobId != 0 {
		for _, jobEntry := range MyCrontab.Entries() {
			if jobEntry.ID == jobId {
				return jobEntry.Next.String()
			}
		}
	}
	return ""
}
