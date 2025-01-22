// Copyright: This file is part of korrel8r, released under https://github.com/korrel8r/korrel8r/blob/main/LICENSE

package k8s

import (
	"testing"

	"github.com/korrel8r/korrel8r/pkg/korrel8r"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/api/meta/testrestmapper"
)

func TestKindToResource(t *testing.T) {
	rm := testrestmapper.TestOnlyStaticRESTMapper(Scheme)
	for _, tc := range [][]string{
		{"pods", "Pod", "v1"},
		{"pods", "Pod", ""},
		{"deployments", "Deployment", "apps/v1"},
		{"events", "Event", "events.k8s.io/v1"},
		{"events", "Event", "v1"},
	} {
		resource, err := kindToResource(rm, tc[1], tc[2])
		if assert.NoError(t, err) {
			assert.Equal(t, tc[0], resource)
		}
	}
	_, err := kindToResource(rm, "x", "y")
	assert.EqualError(t, err, "no matches for kind \"x\" in version \"y\"")
}

func TestK8sClassForKind(t *testing.T) {
	for _, x := range []struct {
		kind string
		want korrel8r.Class
	}{
		{"Pod", Domain.Class("Pod/v1")},
		{"Deployment", Domain.Class("Deployment.apps/v1")},
		{"Event", Domain.Class("Event/v1")},
	} {
		t.Run(x.kind, func(t *testing.T) {
			assert.Equal(t, x.want, k8sClassForKind(x.kind))
		})
	}
}
