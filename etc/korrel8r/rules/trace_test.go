// Copyright: This file is part of korrel8r, released under https://github.com/korrel8r/korrel8r/blob/main/LICENSE

package rules_test

import (
	"testing"

	"github.com/korrel8r/korrel8r/pkg/domains/k8s"
	"github.com/korrel8r/korrel8r/pkg/domains/trace"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Test_TraceToPod(t *testing.T) {
	e := setupT(t)
	for _, x := range []struct {
		rule  string
		start *trace.Span
		want  string
	}{
		{
			rule: "TraceToPod",
			start: &trace.Span{
				Context:    trace.SpanContext{TraceID: "232323", SpanID: "3d48369744164bd0"},
				Attributes: map[string]any{"k8s.namespace.name": "tracing-app-k6", "k8s.pod.name": "bar"},
			},
			want: `k8s:Pod.v1.:{"namespace":"tracing-app-k6","name":"bar"}`,
		},
	} {
		t.Run(x.rule, func(t *testing.T) {
			tested(x.rule)
			got, err := e.Rule(x.rule).Apply(x.start)
			assert.NoError(t, err)
			assert.Equal(t, x.want, got.String())
		})
	}
}

func Test_TraceFromPod(t *testing.T) {
	e := setupT(t)
	for _, x := range []struct {
		rule  string
		start k8s.Object
		want  string
	}{
		{
			rule:  "PodToTrace",
			start: &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "foo", Namespace: "bar"}},
			want:  `trace:span:{resource.k8s.namespace.name="bar"&&resource.k8s.pod.name="foo"}`,
		},
		{
			rule:  "PodToTrace",
			start: &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Namespace: "bar"}},
			want:  `trace:span:{resource.k8s.namespace.name="bar"}`,
		},
		{
			rule:  "PodToTrace",
			start: &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "foo"}},
			want:  `trace:span:{resource.k8s.pod.name="foo"}`,
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
