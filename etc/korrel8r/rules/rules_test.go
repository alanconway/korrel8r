// Copyright: This file is part of korrel8r, released under https://github.com/korrel8r/korrel8r/blob/main/LICENSE

// package rules is a test-only package to unit test YAML rules.
package rules_test

// Test use of rules in graph traversal.

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/korrel8r/korrel8r/internal/pkg/test"
	"github.com/korrel8r/korrel8r/pkg/config"
	"github.com/korrel8r/korrel8r/pkg/domains/alert"
	"github.com/korrel8r/korrel8r/pkg/domains/incident"
	"github.com/korrel8r/korrel8r/pkg/domains/k8s"
	"github.com/korrel8r/korrel8r/pkg/domains/log"
	"github.com/korrel8r/korrel8r/pkg/domains/metric"
	"github.com/korrel8r/korrel8r/pkg/domains/netflow"
	"github.com/korrel8r/korrel8r/pkg/domains/trace"
	"github.com/korrel8r/korrel8r/pkg/engine"
	"github.com/korrel8r/korrel8r/pkg/korrel8r"
	"github.com/korrel8r/korrel8r/pkg/unique"
	"github.com/stretchr/testify/assert"
	"golang.org/x/exp/maps"
	"k8s.io/apimachinery/pkg/api/meta/testrestmapper"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func setup() *engine.Engine {
	configs, err := config.Load("all.yaml")
	if err != nil {
		panic(err)
	}
	for _, c := range configs {
		c.Stores = nil // Use fake stores, not configured defaults.
	}
	c := fake.NewClientBuilder().WithRESTMapper(testrestmapper.TestOnlyStaticRESTMapper(scheme.Scheme)).Build()
	s, err := k8s.NewStore(c, &rest.Config{})
	if err != nil {
		panic(err)
	}
	e, err := engine.Build().
		Domains(k8s.Domain, log.Domain, netflow.Domain, trace.Domain, alert.Domain, metric.Domain, incident.Domain).
		Stores(s). // NOTE: k8s store must come before configs, some templates use k8s functions.
		Config(configs).
		Engine()
	if err != nil {
		panic(err)
	}
	return e
}

func TestMain(m *testing.M) {
	e := setup()
	for _, r := range e.Rules() {
		rules.Add(r.Name())
	}
	m.Run()
	if len(rules) > 0 {
		fmt.Printf("FAIL: %v rules not tested:\n- %v\n", len(rules), strings.Join(maps.Keys(rules), "\n- "))
		os.Exit(1)
	}
}

// tested marks a rule as having been tested.
func tested(ruleName string) { rules.Remove(ruleName) }

var rules = unique.Set[string]{}

type ruleTest struct {
	rule  string
	start korrel8r.Object
	query string
}

func (x ruleTest) Run(t *testing.T) {
	t.Helper()
	t.Run(fmt.Sprintf("%v(%v)", x.rule, test.JSONString(x.start)), func(t *testing.T) {
		t.Helper()
		e := setup()
		r := e.Rule(x.rule)
		if assert.NotNil(t, r, "missing rule: "+x.rule) {
			got, err := r.Apply(x.start)
			if assert.NoError(t, err, x.rule) {
				assert.Equal(t, x.query, got.String())
			}
		}
		tested(x.rule)
	})
}

func newK8s(class, namespace, name string) k8s.Object {
	u := k8s.Wrap(k8s.Domain.Class(class).(k8s.Class).New())
	u.SetNamespace(namespace)
	u.SetName(name)
	return k8s.Unwrap(u)
}

func k8sEvent(o k8s.Object, name string) k8s.Object {
	u := k8s.Wrap(o)
	gvk := u.GetObjectKind().GroupVersionKind()
	e := newK8s("Event", name, u.GetNamespace())
	e["involvedObject"] = k8s.Object{
		"kind":       gvk.Kind,
		"namespace":  u.GetNamespace(),
		"name":       u.GetName(),
		"apiVersion": gvk.GroupVersion().String(),
	}
	return e
}
