/*
Copyright 2023 Avery Blanchard.

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
	"fmt"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"os"
	"os/exec"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"strings"
)

// PodReconciler reconciles a Pod object
type PodReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=pods/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=core,resources=pods/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.14.1/pkg/reconcile
func (r *PodReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = log.FromContext(ctx)

	var pod corev1.Pod
	var status corev1.PodStatus

	if err := r.Get(ctx, req.NamespacedName, &pod); err != nil {
		return ctrl.Result{}, err
	}
	status = pod.Status
	containers := status.ContainerStatuses

	if pod.Labels == nil {
		pod.Labels = make(map[string]string)
	}

	// Iterate over containers
	for _, container := range containers {
		// Parse container ID
		id := container.ContainerID
		split := strings.Split(id, "/")
		id = split[2][:12]

		// Grab container PID from ID
		command := fmt.Sprintf("docker inspect -f '{{.State.Pid}}' %s", id)
		pid, err := exec.Command(command).Output()
		if err != nil {
			return ctrl.Result{}, err
		}
		// Get Linux CGroup from PID
		cgroupPath := fmt.Sprintf("/proc/%d/ns/cgroup", pid)
		symlink, err := os.Readlink(cgroupPath)
		if err != nil {
			return ctrl.Result{}, err
		}

		// Parse NS
		split = strings.Split(symlink, "[")
		split = strings.Split(split[1], "]")
		ns := split[0]

		// Append Pod annotations, mapping container ID to PID and/or NS
		pod.Labels[id] = ns
	}

	// Update pod
	if err := r.Update(ctx, &pod); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *PodReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.Pod{}).
		Complete(r)
}
