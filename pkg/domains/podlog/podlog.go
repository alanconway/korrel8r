// Copyright: This file is part of korrel8r, released under https://github.com/korrel8r/korrel8r/blob/main/LICENSE

// Package podlog provides direct access to Kubernetes Pod logs via the Kube API-server.
//
// Logs are annotated and presented in OTEL log format. FIXME
//
// # Store
//
// The store is the Kube API server itself, providing direct access to live pod log files.
// Logs are not guaranteed to be persisted after a pod is destroyed.
// No parameters are required, a podlog store automatically connects using Kube configuration.
//
//	domain: podlog
package podlog

import (
	"bufio"
	"context"
	"strings"
	"time"

	"github.com/korrel8r/korrel8r/pkg/domains/k8s"
	"github.com/korrel8r/korrel8r/pkg/korrel8r"
	"github.com/korrel8r/korrel8r/pkg/korrel8r/impl"
	"github.com/korrel8r/korrel8r/pkg/result"
	"github.com/korrel8r/korrel8r/pkg/unique"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
)

var (
	// Verify implementing interfaces.
	_ korrel8r.Domain    = Domain
	_ korrel8r.Store     = &Store{}
	_ korrel8r.Query     = Query{}
	_ korrel8r.Class     = Class{}
	_ korrel8r.Previewer = Class{}
)

// FIXME doc
var Domain = &domain{
	impl.NewDomain("podlog", "Live container logs via the Kube API server.", Class{}),
}

type Class struct{}

type Object string

// Query uses the same format as a [k8s.Query] pod query.
type Query struct{ *k8s.Query }

// FIXME
type Store struct {
	s *k8s.Store
	c *corev1client.CoreV1Client
}

type domain struct{ *impl.Domain }

func (d *domain) Query(s string) (korrel8r.Query, error) {
	_, data, err := impl.ParseQuery(d, s)
	if err != nil {
		return nil, err
	}
	kquery, err := k8s.Domain.Query(impl.NameJoin(k8s.Domain.Name(), "Pod.v1", data))
	if err != nil {
		return nil, err
	}
	return Query{kquery.(*k8s.Query)}, nil
}

func (*domain) Store(_ any) (korrel8r.Store, error) {
	s, err := k8s.NewStore(nil, nil)
	if err != nil {
		return nil, err
	}
	c, err := corev1client.NewForConfig(s.Config())
	if err != nil {
		return nil, err
	}
	return &Store{s: s, c: c}, nil
}

// FIXME template this in impl.
func (c Class) Domain() korrel8r.Domain                     { return Domain }
func (c Class) Name() string                                { return "log" }
func (c Class) String() string                              { return impl.ClassString(c) }
func (c Class) Unmarshal(b []byte) (korrel8r.Object, error) { return impl.UnmarshalAs[Object](b) }
func (c Class) Preview(o korrel8r.Object) string            { line, _ := o.(Object); return string(line) }

func (q Query) Class() korrel8r.Class { return Class{} }
func (q Query) String() string        { return impl.QueryString(q) }

func (s *Store) Domain() korrel8r.Domain                 { return Domain }
func (s *Store) StoreClasses() ([]korrel8r.Class, error) { return []korrel8r.Class{Class{}}, nil }

func (s *Store) Get(ctx context.Context, query korrel8r.Query, constraint *korrel8r.Constraint, r korrel8r.Appender) error {
	q, err := impl.TypeAssert[Query](query)
	if err != nil {
		return err
	}

	// Create query options for Pod logs
	opts := &corev1.PodLogOptions{
		Timestamps: true, // FIXME: parse log line for timestamp?
		Container:  "",   // FIXME: include container in search? Include in K8s? (filter pods by container)
	}
	if start := constraint.GetStart(); !start.IsZero() {
		opts.SinceTime = &metav1.Time{Time: *constraint.Start}
	}
	if n := int64(constraint.GetLimit()); n > 0 {
		opts.TailLines = &n
	}
	// FIXME see other fields

	// Get pods using the k8s store.
	pods := result.New(q.Class())
	if err := s.s.Get(ctx, q.Query, constraint, pods); err != nil {
		return err
	}
	errs := unique.Errors{} // Collect partial errors.
	for _, o := range pods.List() {
		pod := k8s.Wrap(o.(k8s.Object))
		req := s.c.Pods(pod.GetNamespace()).GetLogs(pod.GetName(), opts)
		stream, err := req.Stream(context.Background())
		if errs.Add(err) {
			continue
		}
		func() {
			defer stream.Close()
			scanner := bufio.NewScanner(stream)
			for scanner.Scan() {
				timestamp, text, _ := strings.Cut(scanner.Text(), " ")
				o := Object(text)
				if end := constraint.GetEnd(); !end.IsZero() {
					if ts, err := time.Parse(timestamp, time.RFC3339Nano); err == nil && ts.After(end) {
						return
					}
				}
				r.Append(o)
			}
		}()
	}
	return errs.Err()
}
