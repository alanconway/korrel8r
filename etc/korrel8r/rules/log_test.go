// Copyright: This file is part of korrel8r, released under https://github.com/korrel8r/korrel8r/blob/main/LICENSE

package rules_test

import (
	"fmt"
	"testing"

	"github.com/korrel8r/korrel8r/pkg/domains/k8s"
	"github.com/korrel8r/korrel8r/pkg/domains/log"
	"github.com/korrel8r/korrel8r/pkg/korrel8r"
	"github.com/korrel8r/korrel8r/pkg/unique"
	"github.com/stretchr/testify/assert"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestSelectorToLogsRules(t *testing.T) {
	e := setupT(t)
	kd := e.Domain("k8s")
	// Verify rules selected the correct set of start classes
	classes := unique.NewList[korrel8r.Class]()
	for _, r := range e.Rules() {
		if r.Name() == "SelectorToLogs" {
			classes.Append(r.Start()...)
		}
	}
	want := []korrel8r.Class{
		kd.Class("ReplicationController.v1."),
		kd.Class("PersistentVolumeClaim.v1."),
		kd.Class("Service."),
		kd.Class("Deployment.apps"),
		kd.Class("Job.batch"),
		kd.Class("PodDisruptionBudget.policy"),
		kd.Class("StatefulSet.apps"),
		kd.Class("DaemonSet.apps"),
		kd.Class("ReplicaSet.apps"),
	}
	assert.ElementsMatch(t, want, classes.List, "%#v", classes.List)
}

func TestSelectorToLogs(t *testing.T) {
	e := setupT(t)
	kd := e.Domain("k8s")
	d := k8s.New[appsv1.Deployment]("ns", "x")
	d.Spec = appsv1.DeploymentSpec{
		Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"a.b/c": "x"}},
	}
	want := log.NewQuery(log.Application, `{kubernetes_namespace_name="ns"}|json|kubernetes_labels_a_b_c="x"`)
	testTraverse(t, e, kd.Class("Deployment.apps"), log.Domain.Class("application"), []korrel8r.Object{d}, want)
}

func TestPodToLogs(t *testing.T) {
	e := setupT(t)
	kd := e.Domain("k8s")
	for _, pod := range []*corev1.Pod{
		k8s.New[corev1.Pod]("project", "application"),
		k8s.New[corev1.Pod]("kube-something", "infrastructure"),
	} {
		t.Run(pod.Name, func(t *testing.T) {
			want := log.NewQuery(log.Class(pod.Name), fmt.Sprintf(`{kubernetes_namespace_name="%v",kubernetes_pod_name="%v"}`, pod.Namespace, pod.Name))

			testTraverse(t, e, kd.Class("Pod"), log.Domain.Class(pod.Name), []korrel8r.Object{pod}, want)
		})
	}
}
