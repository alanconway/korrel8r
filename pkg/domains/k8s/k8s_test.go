// Copyright: This file is part of korrel8r, released under https://github.com/korrel8r/korrel8r/blob/main/LICENSE

package k8s

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/korrel8r/korrel8r/internal/pkg/test/mock"
	"github.com/korrel8r/korrel8r/pkg/korrel8r"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/meta/testrestmapper"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var (
	d          = NewDomainWith(&rest.Config{}, fake.NewClientBuilder().WithRESTMapper(testrestmapper.TestOnlyStaticRESTMapper(scheme.Scheme)).Build())
	namespace  = &class{d: d, gvk: schema.GroupVersionKind{Kind: "Namespace", Version: "v1"}}
	pod        = &class{d: d, gvk: schema.GroupVersionKind{Kind: "Pod", Version: "v1"}}
	deployment = &class{d: d, gvk: schema.GroupVersionKind{Kind: "Deployment", Version: "v1", Group: "apps"}}
)

func TestDomain_Class_builtin(t *testing.T) {
	for _, x := range []struct {
		name string
		want Class
	}{
		{"Namespace.v1.", namespace},       // Kind, version and group
		{"Namespace.", namespace},          // Kind and version
		{"Namespace", namespace},           // Kind only
		{"Pod.v1.", pod},                   // Kind, version, ""
		{"Pod.v1", pod},                    // Kind, version, implied core group.
		{"Pod.", pod},                      // Kind and group (core group is named "")
		{"Pod", pod},                       // Kind only
		{"Deployment.v1.apps", deployment}, // Kind, version and group
		{"Deployment.apps", deployment},    // Kind and group
		{"Deployment", deployment},         // Kind only
	} {
		t.Run(x.name, func(t *testing.T) {
			got := d.Class(x.name)
			if assert.Equal(t, x.want, got, "from: %v", x.name) {
				// Round trip
				got2 := d.Class(got.Name())
				assert.Equal(t, got, got2)
			}
		})
	}
}

func TestDomain_Class_restmapper(t *testing.T) {
	rm := meta.NewDefaultRESTMapper([]schema.GroupVersion{schema.GroupVersion{Group: "fake", Version: "v1"}})
	dummy := schema.GroupVersionKind{Group: "fake", Version: "v1", Kind: "Dummy"}
	rm.Add(dummy, meta.RESTScopeNamespace)
	d := NewDomainWith(&rest.Config{}, fake.NewClientBuilder().WithRESTMapper(rm).Build())

	for _, x := range []struct {
		name string
		want korrel8r.Class
	}{
		{"Dummy.v1.fake", d.ClassOf(dummy)},
		{"Dummy.fake", d.ClassOf(dummy)},
	} {
		t.Run(x.name, func(t *testing.T) {
			got := d.Class(x.name)
			assert.Equal(t, x.want, got)
			// Round trip
			got2 := d.Class(got.Name())
			assert.Equal(t, got, got2)
		})
	}
}

func TestDomain_Query(t *testing.T) {
	for _, x := range []struct {
		s    string
		want korrel8r.Query
	}{
		{`k8s:Namespace:{"name":"foo"}`, NewQuery(namespace, "", "foo", nil, nil)},
		{`k8s:Namespace:{name: foo}`, NewQuery(namespace, "", "foo", nil, nil)},
		{`k8s:Pod:{namespace: foo, name: bar}`, NewQuery(pod, "foo", "bar", nil, nil)},
		{`k8s:Pod:{namespace: foo, name: bar, labels: { a: b }, fields: { c: d }}`,
			NewQuery(pod, "foo", "bar", map[string]string{"a": "b"}, map[string]string{"c": "d"})},
	} {
		t.Run(x.s, func(t *testing.T) {
			got, err := d.Query(x.s)
			if assert.NoError(t, err) {
				assert.Equal(t, x.want, got)
			}
		})
	}

}

func TestDomain_Query_error(t *testing.T) {
	for _, x := range []struct {
		s   string
		err string
	}{
		// Detect common error: yaml map with missing space interpreted as key containing '"'
		{`k8s:Namespace:{name:"foo"}`, "unknown field"},
	} {
		t.Run(x.s, func(t *testing.T) {
			_, err := d.Query(x.s)
			assert.ErrorContains(t, err, x.err)
		})
	}

}

func TestStore_Get(t *testing.T) {
	c := fake.NewClientBuilder().
		WithRESTMapper(testrestmapper.TestOnlyStaticRESTMapper(scheme.Scheme)).
		WithObjects(
			&corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{Name: "fred", Namespace: "x", Labels: map[string]string{"app": "foo"}},
			},
			&corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{Name: "barney", Namespace: "x", Labels: map[string]string{"app": "bad"}},
			},
			&corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{Name: "wilma", Namespace: "y", Labels: map[string]string{"app": "foo"}},
			},
		).Build()
	d := NewDomainWith(&rest.Config{}, c)
	store, err := d.Store(nil)
	require.NoError(t, err)
	var (
		fred   = types.NamespacedName{Namespace: "x", Name: "fred"}
		barney = types.NamespacedName{Namespace: "x", Name: "barney"}
		wilma  = types.NamespacedName{Namespace: "y", Name: "wilma"}
	)
	for _, x := range []struct {
		q    korrel8r.Query
		want []types.NamespacedName
	}{
		{NewQuery(pod, "x", "fred", nil, nil), []types.NamespacedName{fred}},
		{NewQuery(pod, "x", "", nil, nil), []types.NamespacedName{fred, barney}},
		{NewQuery(pod, "", "", client.MatchingLabels{"app": "foo"}, nil), []types.NamespacedName{fred, wilma}},
	} {
		t.Run(fmt.Sprintf("%#v", x.q), func(t *testing.T) {
			var result mock.Result
			err = store.Get(context.Background(), x.q, nil, &result)
			require.NoError(t, err)
			var got []types.NamespacedName
			for _, v := range result {
				o := v.(Object)
				got = append(got, types.NamespacedName{
					Namespace: UnstructuredOf(o).GetNamespace(),
					Name:      UnstructuredOf(o).GetName(),
				})
			}
			assert.ElementsMatch(t, x.want, got)
		})
	}
	// Need to validate labels and all get variations on fake client or env test...
}

func TestStore_Get_Constraint(t *testing.T) {
	// Time range [start,end] and some time points.
	start := time.Now()
	end := start.Add(time.Minute)
	testPod := func(name string, t time.Time) *corev1.Pod {
		return &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "test", CreationTimestamp: metav1.Time{Time: t}},
		}
	}
	early, ontime, late := testPod("early", start.Add(-time.Second)), testPod("ontime", start.Add(time.Second)), testPod("late", end.Add(time.Second))
	c := fake.NewClientBuilder().
		WithRESTMapper(testrestmapper.TestOnlyStaticRESTMapper(scheme.Scheme)).
		WithObjects(early, ontime, late).Build()
	d := NewDomainWith(&rest.Config{}, c)
	store, err := d.Store(nil)
	require.NoError(t, err)

	for _, x := range []struct {
		constraint *korrel8r.Constraint
		want       []string
	}{
		{&korrel8r.Constraint{Start: &start, End: &end}, []string{"early", "ontime"}},
		{&korrel8r.Constraint{Start: &start}, []string{"early", "ontime", "late"}},
		{&korrel8r.Constraint{End: &end}, []string{"early", "ontime"}},
		{nil, []string{"early", "ontime", "late"}},
	} {
		t.Run(fmt.Sprintf("%+v", x.constraint), func(t *testing.T) {
			var result mock.Result
			err = store.Get(context.Background(), NewQuery(pod, "test", "", nil, nil), x.constraint, &result)
			require.NoError(t, err)
			var got []string
			for _, v := range result {
				got = append(got, UnstructuredOf(v.(Object)).GetName())
			}
			assert.ElementsMatch(t, x.want, got, "%v != %v", x.want, got)
		})
	}
	// Need to validate labels and all get variations on fake client or env test...
}
