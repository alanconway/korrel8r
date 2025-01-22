// Copyright: This file is part of korrel8r, released under https://github.com/korrel8r/korrel8r/blob/main/LICENSE

package rules_test

import (
	"testing"

	"github.com/korrel8r/korrel8r/pkg/domains/k8s"
	"github.com/korrel8r/korrel8r/pkg/domains/netflow"
	"github.com/stretchr/testify/assert"
)

func Test_NetflowToK8S(t *testing.T) {
	e := setupT(t)
	for _, x := range []struct {
		rule  string
		start netflow.Object
		want  string
	}{
		{
			rule:  "NetflowToSrcK8s",
			start: netflow.Object{"SrcK8S_Type": "Pod", "SrcK8S_Namespace": "foo", "SrcK8S_Name": "bar"},
			want:  `k8s:Pod.v1.:{"namespace":"foo","name":"bar"}`,
		},
		{
			rule:  "NetflowToSrcK8sOwner",
			start: netflow.Object{"SrcK8S_OwnerType": "Deployment", "SrcK8S_Namespace": "foo", "SrcK8S_OwnerName": "bar"},
			want:  `k8s:Deployment.v1.apps:{"namespace":"foo","name":"bar"}`,
		},
		{
			rule:  "NetflowToDstK8s",
			start: netflow.Object{"DstK8S_Type": "Pod", "DstK8S_Namespace": "foo", "DstK8S_Name": "bar"},
			want:  `k8s:Pod.v1.:{"namespace":"foo","name":"bar"}`,
		},
		{
			rule:  "NetflowToDstK8sOwner",
			start: netflow.Object{"DstK8S_OwnerType": "Deployment", "DstK8S_Namespace": "foo", "DstK8S_OwnerName": "bar"},
			want:  `k8s:Deployment.v1.apps:{"namespace":"foo","name":"bar"}`,
		},
	} {
		t.Run(x.rule, func(t *testing.T) {
			tested(x.rule)
			got, err := e.Rule(x.rule).Apply(x.start)
			if assert.NoError(t, err) {
				assert.Equal(t, x.want, got.String())
			}
		})
	}
}

func Test_NetflowToK8S_skipped(t *testing.T) {
	// Get expected error when fields are missing.
	e := setupT(t)
	for _, x := range []struct {
		rule  string
		start netflow.Object
		want  string
	}{
		{
			rule:  "NetflowToSrcK8s",
			start: netflow.Object{"SrcK8S_Namespace": "foo", "SrcK8S_Name": "bar"},
			want:  `k8s:Pod.v1.:{"namespace":"foo","name":"bar"}`,
		},
		{
			rule:  "NetflowToSrcK8sOwner",
			start: netflow.Object{"SrcK8S_Namespace": "foo", "SrcK8S_OwnerName": "bar"},
			want:  `k8s:Deployment.v1.apps:{"namespace":"foo","name":"bar"}`,
		},
		{
			rule:  "NetflowToDstK8s",
			start: netflow.Object{"DstK8S_Namespace": "foo", "DstK8S_Name": "bar"},
			want:  `k8s:Pod.v1.:{"namespace":"foo","name":"bar"}`,
		},
		{
			rule:  "NetflowToDstK8sOwner",
			start: netflow.Object{"DstK8S_Namespace": "foo", "DstK8S_OwnerName": "bar"},
			want:  `k8s:Deployment.v1.apps:{"namespace":"foo","name":"bar"}`,
		},
	} {
		t.Run(x.rule, func(t *testing.T) {
			tested(x.rule)
			r := e.Rule(x.rule)
			got, err := r.Apply(x.start)
			assert.ErrorContains(t, err, "map has no entry")
			assert.Nil(t, got)
		})
	}
}

func Test_NetflowFromK8S(t *testing.T) {
	e := setupT(t)
	foobar := k8s.Object{"name": "foo", "namespace": "bar"}
	for _, x := range []struct {
		rule  string
		start k8s.Object
		want  string
	}{
		{
			rule:  "K8sSrcToNetflow",
			start: k8s.Object{"kind": "Pod", "metadata": foobar},
			want:  `netflow:network:{SrcK8S_Type="", SrcK8S_Namespace="bar"} | json | SrcK8S_Name="foo"`,
		},
		{
			rule:  "K8sSrcOwnerToNetflow",
			start: k8s.Object{"kind": "Deployment.app", "metadata": foobar},
			want:  `netflow:network:{SrcK8S_Namespace="bar", SrcK8S_OwnerName="foo"}`,
		},
		{
			rule:  "K8sDstToNetflow",
			start: k8s.Object{"kind": "Pod", "metadata": foobar},
			want:  `netflow:network:{DstK8S_Type="", DstK8S_Namespace="bar"} | json | DstK8S_Name="foo"`,
		},
		{
			rule:  "K8sDstOwnerToNetflow",
			start: k8s.Object{"kind": "Deployment.app", "metadata": foobar},
			want:  `netflow:network:{DstK8S_Namespace="bar", DstK8S_OwnerName="foo"}`,
		},
	} {
		t.Run(x.rule, func(t *testing.T) {
			tested(x.rule)
			got, err := e.Rule(x.rule).Apply(x.start)
			if assert.NoError(t, err) {
				assert.Equal(t, x.want, got.String())
			}
		})
	}
}
