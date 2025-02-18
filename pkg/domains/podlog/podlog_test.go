package podlog_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/korrel8r/korrel8r/internal/pkg/test"
	"github.com/korrel8r/korrel8r/pkg/domains/podlog"
	"github.com/korrel8r/korrel8r/pkg/korrel8r"
	"github.com/korrel8r/korrel8r/pkg/ptr"
	"github.com/korrel8r/korrel8r/pkg/result"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestGetPodLogs(t *testing.T) {
	test.SkipIfNoCluster(t)
	kt := test.NewK8sClient(t)
	const n = 3
	logger(kt, "foo", "hello", n)
	logger(kt, "bar", "goodbye", n)
	s, err := podlog.Domain.Store(nil)
	require.NoError(kt, err)

	kt.Run("multi pod", func(t *testing.T) {
		got := getLogs(t, fmt.Sprintf("podlog:log:{namespace: %v}", kt.Namespace), s)
		want := make([]korrel8r.Object, 2*n)
		for i := 0; i < n; i++ {
			want[i] = podlog.Object(fmt.Sprintf("hello %v", i+1))
			want[i+n] = podlog.Object(fmt.Sprintf("goodbye %v", i+1))
		}
		assert.ElementsMatch(t, want, got)
	})

	kt.Run("single pod", func(t *testing.T) {
		got := getLogs(t, fmt.Sprintf("podlog:log:{name: foo, namespace: %v}", kt.Namespace), s)
		want := make([]korrel8r.Object, n)
		for i := range want {
			want[i] = podlog.Object(fmt.Sprintf("hello %v", i+1))
		}
		assert.Equal(t, want, got)
	})
}

func getLogs(t *testing.T, query string, s korrel8r.Store) []korrel8r.Object {
	t.Helper()
	q, err := podlog.Domain.Query(query)
	require.NoError(t, err)
	r := result.New(q.Class())
	assert.Eventually(t, func() bool {
		err = s.Get(context.Background(), q, nil, r)
		return err == nil
	}, time.Second*30, time.Second)
	assert.NoError(t, err)
	return r.List()
}

func logger(t *test.K8sClient, name string, text string, n int) {
	pod := &corev1.Pod{
		ObjectMeta: v1.ObjectMeta{Namespace: t.Namespace, Name: name},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{
				Name:  "logger",
				Image: "quay.io/quay/busybox",
				Command: []string{"sh", "-c",
					fmt.Sprintf(`for i in $(seq %v); do echo "%v $i"; done`, n, text)},
				SecurityContext: &corev1.SecurityContext{
					SeccompProfile: &corev1.SeccompProfile{
						Type: corev1.SeccompProfileTypeRuntimeDefault,
					},
					AllowPrivilegeEscalation: ptr.To(false),
					Capabilities: &corev1.Capabilities{
						Drop: []corev1.Capability{"ALL"},
					},
				},
			}},
		},
	}
	require.NoError(t, t.Create(context.Background(), pod))
}

// FIXME constraints: begin, end, limit
