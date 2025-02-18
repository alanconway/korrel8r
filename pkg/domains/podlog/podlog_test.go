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
	ct := test.NewK8sClientT(t)
	ns := ct.Namespace
	s, err := podlog.Domain.Store(nil)
	require.NoError(t, err)

	const n = 10
	logger(ct, "foo", "hello", n, 0)
	logger(ct, "bar", "goodbye", n, 0)

	t.Run("multi pod", func(t *testing.T) {
		got := getLogs(t, s, fmt.Sprintf("podlog:log:{namespace: %v}", ns), nil, n)
		want := append(wantLogs("hello", 1, n), wantLogs("goodbye", 1, n)...)
		assert.ElementsMatch(t, want, got)
	})

	fooQuery := fmt.Sprintf("podlog:log:{name: foo, namespace: %v}", ns)

	t.Run("single pod", func(t *testing.T) {
		got := getLogs(t, s, fooQuery, nil, n)
		assert.Equal(t, wantLogs("hello", 1, n), got)
	})

	t.Run("constraint limit", func(t *testing.T) {
		got := getLogs(t, s, fooQuery, &korrel8r.Constraint{Limit: ptr.To(5)}, 5)
		assert.Equal(t, wantLogs("hello", 6, 10), got)
	})

	// FIXME test time constraints, need timestamps.
}

func wantLogs(text string, a, b int) []korrel8r.Object {
	n := 1 + b - a
	want := make([]korrel8r.Object, n)
	for i := range want {
		want[i] = podlog.Object(fmt.Sprintf("%v %v", text, a+i))
	}
	return want
}

func getLogs(t testing.TB, s korrel8r.Store, query string, constraint *korrel8r.Constraint, min int) (logs []korrel8r.Object) {
	t.Helper()
	q, err := podlog.Domain.Query(query)
	require.NoError(t, err)
	assert.Eventually(t, func() bool {
		r := result.New(q.Class())
		err = s.Get(context.Background(), q, constraint, r)
		logs = r.List()
		return err == nil && len(logs) >= min
	}, time.Second*30, time.Second)
	assert.NoError(t, err)
	return logs
}

func logger(t *test.K8sClientT, name string, text string, n int, pause float32) {
	pod := &corev1.Pod{
		ObjectMeta: v1.ObjectMeta{Namespace: t.Namespace, Name: name},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{
				Name:  "logger",
				Image: "quay.io/quay/busybox",
				Command: []string{"sh", "-c",
					fmt.Sprintf(`for i in $(seq %v); do echo "%v $i"; sleep %v; done`, n, text, pause)},
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
