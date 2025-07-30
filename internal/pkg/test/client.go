// Copyright: This file is part of korrel8r, released under https://github.com/korrel8r/korrel8r/blob/main/LICENSE

package test

import (
	"context"
	"testing"

	"github.com/korrel8r/korrel8r/pkg/domains/k8s"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type K8sClientT struct {
	*testing.T
	client.Client
	Namespace string
}

func NewK8sClientT(t *testing.T) *K8sClientT {
	c, err := k8s.NewClient(nil)
	require.NoError(t, err)
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "test-" + RandomName(8),
			Labels: map[string]string{"test": "test"},
		},
	}
	require.NoError(t, c.Create(context.Background(), ns))
	t.Cleanup(func() { _ = c.Delete(context.Background(), ns) })
	return &K8sClientT{T: t, Client: c, Namespace: ns.Name}
}
