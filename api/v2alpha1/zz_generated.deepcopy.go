//go:build !ignore_autogenerated
// +build !ignore_autogenerated

//
// Copyright (c) 2019-2021 Red Hat, Inc.
// This program and the accompanying materials are made
// available under the terms of the Eclipse Public License 2.0
// which is available at https://www.eclipse.org/legal/epl-2.0/
//
// SPDX-License-Identifier: EPL-2.0
//
// Contributors:
//   Red Hat, Inc. - initial API and implementation
//

// Code generated by controller-gen. DO NOT EDIT.

package v2alpha1

import (
	"k8s.io/apimachinery/pkg/labels"
	runtime "k8s.io/apimachinery/pkg/runtime"
)

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *CheCluster) DeepCopyInto(out *CheCluster) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	out.Status = in.Status
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new CheCluster.
func (in *CheCluster) DeepCopy() *CheCluster {
	if in == nil {
		return nil
	}
	out := new(CheCluster)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *CheCluster) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *CheClusterList) DeepCopyInto(out *CheClusterList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]CheCluster, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new CheClusterList.
func (in *CheClusterList) DeepCopy() *CheClusterList {
	if in == nil {
		return nil
	}
	out := new(CheClusterList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *CheClusterList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *CheClusterSpec) DeepCopyInto(out *CheClusterSpec) {
	*out = *in
	if in.Enabled != nil {
		in, out := &in.Enabled, &out.Enabled
		*out = new(bool)
		**out = **in
	}
	out.WorkspaceDomainEndpoints = in.WorkspaceDomainEndpoints
	in.Gateway.DeepCopyInto(&out.Gateway)
	in.K8s.DeepCopyInto(&out.K8s)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new CheClusterSpec.
func (in *CheClusterSpec) DeepCopy() *CheClusterSpec {
	if in == nil {
		return nil
	}
	out := new(CheClusterSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *CheClusterSpecK8s) DeepCopyInto(out *CheClusterSpecK8s) {
	*out = *in
	if in.IngressAnnotations != nil {
		in, out := &in.IngressAnnotations, &out.IngressAnnotations
		*out = make(map[string]string, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new CheClusterSpecK8s.
func (in *CheClusterSpecK8s) DeepCopy() *CheClusterSpecK8s {
	if in == nil {
		return nil
	}
	out := new(CheClusterSpecK8s)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *CheClusterStatusV2Alpha1) DeepCopyInto(out *CheClusterStatusV2Alpha1) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new CheClusterStatusV2Alpha1.
func (in *CheClusterStatusV2Alpha1) DeepCopy() *CheClusterStatusV2Alpha1 {
	if in == nil {
		return nil
	}
	out := new(CheClusterStatusV2Alpha1)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *CheGatewaySpec) DeepCopyInto(out *CheGatewaySpec) {
	*out = *in
	if in.Enabled != nil {
		in, out := &in.Enabled, &out.Enabled
		*out = new(bool)
		**out = **in
	}
	if in.ConfigLabels != nil {
		in, out := &in.ConfigLabels, &out.ConfigLabels
		*out = make(labels.Set, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new CheGatewaySpec.
func (in *CheGatewaySpec) DeepCopy() *CheGatewaySpec {
	if in == nil {
		return nil
	}
	out := new(CheGatewaySpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *WorkspaceDomainEndpoints) DeepCopyInto(out *WorkspaceDomainEndpoints) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new WorkspaceDomainEndpoints.
func (in *WorkspaceDomainEndpoints) DeepCopy() *WorkspaceDomainEndpoints {
	if in == nil {
		return nil
	}
	out := new(WorkspaceDomainEndpoints)
	in.DeepCopyInto(out)
	return out
}
