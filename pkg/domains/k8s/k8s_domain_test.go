// Copyright: This file is part of korrel8r, released under https://github.com/korrel8r/korrel8r/blob/main/LICENSE

package k8s_test

import (
	"testing"

	"github.com/korrel8r/korrel8r/internal/pkg/test/domain"
	"github.com/korrel8r/korrel8r/pkg/domains/k8s"
	"k8s.io/apimachinery/pkg/api/meta/testrestmapper"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var (
	d = k8s.NewTestDomain(&rest.Config{},
		fake.NewClientBuilder().WithRESTMapper(testrestmapper.TestOnlyStaticRESTMapper(scheme.Scheme)).Build(), nil)
	fixture = domain.Fixture{Query: k8s.NewQuery(d.Class("Pod").(k8s.Class), "", "", nil, nil)}
)

func TestK8sDomain(t *testing.T)      { fixture.Test(t) }
func BenchmarkK8sDomain(b *testing.B) { fixture.Benchmark(b) }
