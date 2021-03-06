/*
Copyright 2019 The Kubernetes Authors.

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

package volumebinding

import (
	"context"

	v1 "k8s.io/api/core/v1"
	framework "k8s.io/kubernetes/pkg/scheduler/framework/v1alpha1"
	schedulernodeinfo "k8s.io/kubernetes/pkg/scheduler/nodeinfo"
	"k8s.io/kubernetes/pkg/scheduler/volumebinder"
)

// VolumeBinding is a plugin that binds pod volumes in scheduling.
type VolumeBinding struct {
	binder *volumebinder.VolumeBinder
}

var _ framework.FilterPlugin = &VolumeBinding{}

// Name is the name of the plugin used in Registry and configurations.
const Name = "VolumeBinding"

const (
	// ErrReasonBindConflict is used for VolumeBindingNoMatch predicate error.
	ErrReasonBindConflict = "node(s) didn't find available persistent volumes to bind"
	// ErrReasonNodeConflict is used for VolumeNodeAffinityConflict predicate error.
	ErrReasonNodeConflict = "node(s) had volume node affinity conflict"
)

// Name returns name of the plugin. It is used in logs, etc.
func (pl *VolumeBinding) Name() string {
	return Name
}

func podHasPVCs(pod *v1.Pod) bool {
	for _, vol := range pod.Spec.Volumes {
		if vol.PersistentVolumeClaim != nil {
			return true
		}
	}
	return false
}

// Filter invoked at the filter extension point.
// It evaluates if a pod can fit due to the volumes it requests,
// for both bound and unbound PVCs.
//
// For PVCs that are bound, then it checks that the corresponding PV's node affinity is
// satisfied by the given node.
//
// For PVCs that are unbound, it tries to find available PVs that can satisfy the PVC requirements
// and that the PV node affinity is satisfied by the given node.
//
// The predicate returns true if all bound PVCs have compatible PVs with the node, and if all unbound
// PVCs can be matched with an available and node-compatible PV.
func (pl *VolumeBinding) Filter(ctx context.Context, cs *framework.CycleState, pod *v1.Pod, nodeInfo *schedulernodeinfo.NodeInfo) *framework.Status {
	node := nodeInfo.Node()
	if node == nil {
		return framework.NewStatus(framework.Error, "node not found")
	}
	// If pod does not request any PVC, we don't need to do anything.
	if !podHasPVCs(pod) {
		return nil
	}

	unboundSatisfied, boundSatisfied, err := pl.binder.Binder.FindPodVolumes(pod, node)

	if err != nil {
		return framework.NewStatus(framework.Error, err.Error())
	}

	if !boundSatisfied || !unboundSatisfied {
		status := framework.NewStatus(framework.UnschedulableAndUnresolvable)
		if !boundSatisfied {
			status.AppendReason(ErrReasonNodeConflict)
		}
		if !unboundSatisfied {
			status.AppendReason(ErrReasonBindConflict)
		}
		return status
	}
	return nil
}

// NewFromVolumeBinder initializes a new plugin with volume binder and returns it.
func NewFromVolumeBinder(volumeBinder *volumebinder.VolumeBinder) framework.Plugin {
	return &VolumeBinding{
		binder: volumeBinder,
	}
}
