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
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"github.com/go-logr/logr"
	"github.com/robfig/cron/v3"
	leisurev1beta1 "github.com/shuhanghang/kube-leisure/api/v1beta1"
)

var MyCrontab = cron.New()

var MyCronCache = map[string]map[string]cron.EntryID{}

// LeisureReconciler reconciles a Leisure object
type LeisureReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	log    logr.Logger
}

type RestartGroup struct {
	Name    string
	Req     ctrl.Request
	ObjMeta RestartObjMeta
}

type RestartObjMeta struct {
	Name      string
	NameSpace string
	Crontab   string
	Type      string
	Id        string
}

type CronJobsCache map[string]map[string]int32
type CronJobCache map[string]int32

//+kubebuilder:rbac:groups=leisure.shuhanghang.com,resources=leisures,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=leisure.shuhanghang.com,resources=leisures/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=leisure.shuhanghang.com,resources=leisures/finalizers,verbs=update

func (r *LeisureReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	r.log = log.FromContext(ctx)
	var restartGroup = RestartGroup{
		Name: req.NamespacedName.String(),
		Req:  req,
	}
	leisureRestart := leisurev1beta1.Leisure{}

	err := r.Get(context.TODO(), req.NamespacedName, &leisureRestart)
	if err != nil {
		if errors.IsNotFound(err) {
			if !reflect.DeepEqual(MyCronCache[restartGroup.Name], CronJobCache{}) {
				jobs := MyCronCache[restartGroup.Name]
				for _, job := range jobs {
					MyCrontab.Remove(job)
					r.log.Info("The job has been deleted", "CronID", job, "Lens of crontab entries", len(MyCrontab.Entries()))
				}
			} else {
				r.log.Info("Not found leisure object, it may be removed")
			}
			return ctrl.Result{}, nil
		}
	}

	restartObj := RestartObjMeta{
		Name:      leisureRestart.Spec.Restart.Name,
		NameSpace: leisureRestart.Spec.Restart.NameSpace,
		Crontab:   "CRON_TZ=" + leisureRestart.Spec.Restart.TimeZone + " " + leisureRestart.Spec.Restart.RestartAt,
		Type:      leisureRestart.Spec.Restart.ResourceType,
	}

	restartObj.Id = restartObj.Type + "-" + restartObj.Name + "-" + restartObj.NameSpace

	restartGroup.ObjMeta = restartObj

	// Delete job
	if !reflect.DeepEqual(MyCronCache[restartGroup.Name], CronJobCache{}) {
		jobId, found := MyCronCache[restartGroup.Name][restartGroup.ObjMeta.Id]
		if found {
			MyCrontab.Remove(jobId)
			r.log.Info("The job has been deleted", "CronID", restartGroup.Name, "Lens of cron entries", len(MyCrontab.Entries()))
		}

		r.log.Info("Restart job has been checked", "JobMeta", restartGroup.ObjMeta)
	}

	// Add job
	if restartGroup.ObjMeta.Type == "deployment" {
		ei, _ := MyCrontab.AddFunc(restartGroup.ObjMeta.Crontab, func() { r.AddRolloutRestartDeploy(restartGroup) })
		MyCronCache[restartGroup.Name] = map[string]cron.EntryID{restartGroup.ObjMeta.Id: ei}
		r.log.Info("Launch the job and set cronId to cronCache", "CronID", ei)

	} else if restartGroup.ObjMeta.Type == "statefulset" {
		ei, _ := MyCrontab.AddFunc(restartGroup.ObjMeta.Crontab, func() { r.AddRolloutRestartStatefulset(restartGroup) })
		MyCronCache[restartGroup.Name] = map[string]cron.EntryID{restartGroup.ObjMeta.Id: ei}
		r.log.Info("Launch the job and set cronId to cronCache", "CronID", ei)

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

func (r *LeisureReconciler) AddRolloutRestartDeploy(restartDep RestartGroup) {
	dep := appsv1.Deployment{}
	specDeployment := types.NamespacedName{
		Name:      restartDep.ObjMeta.Name,
		Namespace: restartDep.ObjMeta.NameSpace,
	}
	err := r.Get(context.TODO(), specDeployment, &dep)
	if err != nil {
		r.log.Error(err, "error to get the deployment", "deployment", specDeployment)
	}
	if reflect.DeepEqual(dep, appsv1.Deployment{}) {
		r.log.Info("Not found the deployment", "CrdName", specDeployment.Name, "CrdNameSpace", specDeployment.Namespace)
		return
	}
	patch := client.MergeFrom(dep.DeepCopy())
	restartAt := time.Now().Format("2006-01-02 15:04:05")
	dep.Spec.Template.Annotations = map[string]string{}
	dep.Spec.Template.Annotations["kubectl.kubernetes.io/restartedAt"] = restartAt
	err = r.Patch(context.TODO(), &dep, patch)
	if err != nil {
		r.log.Error(err, "error to restart the Deployment", "deployment", specDeployment)
	}
	r.UpdateStatus(restartDep)
	r.log.Info("Deployment has being restarted", "RestartAt", restartAt, "NextAt", GetJobNextAt(restartDep), "RestartObj", specDeployment)
}

func (r *LeisureReconciler) AddRolloutRestartStatefulset(restartState RestartGroup) {
	dep := appsv1.StatefulSet{}
	specStatefulset := types.NamespacedName{
		Name:      restartState.ObjMeta.Name,
		Namespace: restartState.ObjMeta.NameSpace,
	}
	err := r.Get(context.TODO(), specStatefulset, &dep)
	if err != nil {
		r.log.Error(err, "error to get the StatefulSet", "StatefulSet", specStatefulset)
	}
	if reflect.DeepEqual(dep, appsv1.StatefulSet{}) {
		r.log.Info("Not found the statefulset", "CrdName", specStatefulset.Name, "CrdNameSpace", specStatefulset.Namespace)
		return
	}
	patch := client.MergeFrom(dep.DeepCopy())
	restartAt := time.Now().Format("2006-01-02 15:04:05")
	dep.Spec.Template.Annotations = map[string]string{}
	dep.Spec.Template.Annotations["kubectl.kubernetes.io/restartedAt"] = restartAt
	err = r.Patch(context.TODO(), &dep, patch)
	if err != nil {
		r.log.Error(err, "error to restart the StatefulSet", "StatefulSet", specStatefulset)
	}
	r.UpdateStatus(restartState)
	r.log.Info("Statefuleset has being restarted", "RestartAt", restartAt, "NextAt", GetJobNextAt(restartState), "RestartObj", specStatefulset)

}

// Update Status
func (r *LeisureReconciler) UpdateStatus(restartObj RestartGroup) {
	leisure := leisurev1beta1.Leisure{}
	err := r.Get(context.TODO(), restartObj.Req.NamespacedName, &leisure)
	if err == nil {
		leisure.Status.NextAt = GetJobNextAt(restartObj)
		r.Status().Update(context.TODO(), &leisure)
	}
}

// Get job Next time
func GetJobNextAt(restartObj RestartGroup) string {
	jobId, found := MyCronCache[restartObj.Name][restartObj.ObjMeta.Id]
	if found {
		for _, jobEntry := range MyCrontab.Entries() {
			if jobEntry.ID == jobId {
				return jobEntry.Next.String()
			}
		}
	}
	return ""
}
