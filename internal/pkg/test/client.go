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

type K8sClient struct {
	*testing.T
	client.Client
	Namespace string
}

func NewK8sClient(t *testing.T) *K8sClient {
	c, err := k8s.NewClient(nil)
	require.NoError(t, err)
	ct := &K8sClient{T: t, Client: c, Namespace: "test-" + RandomName(8)}
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name:   ct.Namespace,
			Labels: map[string]string{"test": "test"},
		},
	}
	require.NoError(t, c.Create(context.Background(), ns))
	t.Cleanup(func() { c.Delete(context.Background(), ns) })
	return ct
}
