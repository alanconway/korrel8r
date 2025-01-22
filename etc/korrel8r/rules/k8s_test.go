// Copyright: This file is part of korrel8r, released under https://github.com/korrel8r/korrel8r/blob/main/LICENSE

package rules_test

import (
	"testing"

	"github.com/korrel8r/korrel8r/pkg/domains/alert"
	"github.com/korrel8r/korrel8r/pkg/domains/k8s"
	"github.com/korrel8r/korrel8r/pkg/domains/log"
	"github.com/korrel8r/korrel8r/pkg/domains/metric"
	"github.com/korrel8r/korrel8r/pkg/korrel8r"
	"github.com/stretchr/testify/assert"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestLogToPod(t *testing.T) {
	e := setupT(t)
	kd := e.Domain("k8s")
	r := e.Rule("LogToPod")
	for _, o := range []log.Object{
		log.NewObject(`{"kubernetes":{"namespace_name":"foo","pod_name":"bar"}, "message":"hello"}`),
		log.NewObject(`{"kubernetes":{"namespace_name":"default","pod_name":"baz"}, "message":"bye"}`),
	} {
		t.Run(log.Preview(o), func(t *testing.T) {
			k := o["kubernetes"].(map[string]any)
			namespace := k["namespace_name"].(string)
			name := k["pod_name"].(string)
			pod := kd.Class("Pod")
			want := k8s.NewQuery(pod.(k8s.Class), namespace, name, nil, nil)
			q, err := r.Apply(o)
			if assert.NoError(t, err) {
				assert.Equal(t, want, q)
			}
		})
	}
}

func TestSelectorToPods(t *testing.T) {
	e := setupT(t)

	// Deployment
	labels := map[string]string{"test": "testme"}

	podx := k8s.New[corev1.Pod]("ns", "x")
	podx.ObjectMeta.Labels = labels
	podx.Spec = corev1.PodSpec{
		Containers: []corev1.Container{{
			Name:    "testme",
			Image:   "quay.io/quay/busybox",
			Command: []string{"sh", "-c", "while true; do echo $(date) hello world; sleep 1; done"},
		}}}

	pody := podx.DeepCopy()
	pody.ObjectMeta.Name = "y"

	d := k8s.New[appsv1.Deployment]("ns", "x")
	d.Spec = appsv1.DeploymentSpec{
		Selector: &metav1.LabelSelector{MatchLabels: labels},
		Template: corev1.PodTemplateSpec{
			ObjectMeta: podx.ObjectMeta,
			Spec:       podx.Spec,
		}}
	kd := e.Domain("k8s")
	class := kd.Class("Pod")
	testTraverse(t, e, kd.Class("Deployment.apps"), class, []korrel8r.Object{k8s.ObjectOf(d)},
		k8s.NewQuery(class.(k8s.Class), "ns", "", client.MatchingLabels{"test": "testme"}, nil))
}

func TestK8sEvent(t *testing.T) {
	e := setupT(t)
	kd := e.Domain("k8s")
	pod := k8s.New[corev1.Pod]("aNamespace", "foo")
	event := k8s.EventFor(pod, "a")

	t.Run("PodToEvent", func(t *testing.T) {
		want := k8s.NewQuery(
			kd.Class("Event.v1.").(k8s.Class), "", "", nil,
			client.MatchingFields{
				"involvedObject.apiVersion": "v1", "involvedObject.kind": "Pod",
				"involvedObject.name": "foo", "involvedObject.namespace": "aNamespace"})
		testTraverse(t, e, kd.Class("Pod"), kd.Class("Event.v1."), []korrel8r.Object{pod}, want)
	})
	t.Run("EventToPod", func(t *testing.T) {
		want := k8s.NewQuery(kd.Class("Pod").(k8s.Class), "aNamespace", "foo", nil, nil)
		testTraverse(t, e, kd.Class("Event.v1."), kd.Class("Pod"), []korrel8r.Object{event}, want)
	})
}

func TestK8sAllToMetric(t *testing.T) {
	e := setupT(t)
	kd := e.Domain("k8s")
	pod := k8s.New[corev1.Pod]("aNamespace", "foo")
	want := metric.Query("{namespace=\"aNamespace\",pod=\"foo\"}")
	testTraverse(t, e, kd.Class("Pod"), want.Class(), []korrel8r.Object{pod}, want)
}

func TestK8sPOdToAlert(t *testing.T) {
	e := setupT(t)
	kd := e.Domain("k8s")
	pod := k8s.New[corev1.Pod]("aNamespace", "foo")
	want := alert.Query{"namespace": "aNamespace", "pod": "foo"}
	testTraverse(t, e, kd.Class("Pod"), want.Class(), []korrel8r.Object{pod}, want)
}
