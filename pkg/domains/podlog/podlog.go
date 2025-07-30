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
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/korrel8r/korrel8r/pkg/domains/k8s"
	"github.com/korrel8r/korrel8r/pkg/korrel8r"
	"github.com/korrel8r/korrel8r/pkg/korrel8r/impl"
	"github.com/korrel8r/korrel8r/pkg/otel"
	"github.com/korrel8r/korrel8r/pkg/result"
	"github.com/korrel8r/korrel8r/pkg/unique"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	// Verify implementing interfaces.
	_ korrel8r.Domain    = Domain
	_ korrel8r.Store     = &Store{}
	_ korrel8r.Query     = &Query{}
	_ korrel8r.Class     = Class{}
	_ korrel8r.Previewer = Class{}
)

// FIXME doc everything

var Domain = &domain{
	impl.NewDomain("podlog", "Live container logs via the Kube API server.", Class{}),
}

type Class struct{}

type Object otel.Log

// Query uses the same format as a [k8s.Query] with and additional 'container' field.
type Query struct {
	k8sQuery *k8s.Query
	Selector
}

type Selector struct {
	k8s.Selector `json:",inline"`
	Container    string `json:"container,omitempty"`
}

type Store struct {
	s *k8s.Store
	c *corev1client.CoreV1Client
}

type domain struct{ *impl.Domain }

func (d *domain) Query(s string) (korrel8r.Query, error) {
	_, selector, err := impl.UnmarshalQueryString[Selector](d, s)
	if err != nil {
		return nil, err
	}
	return &Query{
		k8sQuery: k8s.NewQuery(k8s.ClassNamed("Pod.v1"), selector.Selector),
		Selector: selector,
	}, nil
}

func (*domain) Store(_ any) (korrel8r.Store, error) { return NewStore(nil, nil) }

func NewStore(c client.Client, cfg *rest.Config) (*Store, error) {
	s, err := k8s.NewStore(c, cfg)
	if err != nil {
		return nil, err
	}
	cc, err := corev1client.NewForConfig(s.Config())
	if err != nil {
		return nil, err
	}
	return &Store{s: s, c: cc}, nil
}

func (c Class) Domain() korrel8r.Domain                     { return Domain }
func (c Class) Name() string                                { return "log" }
func (c Class) String() string                              { return impl.ClassString(c) }
func (c Class) Unmarshal(b []byte) (korrel8r.Object, error) { return impl.UnmarshalAs[Object](b) }
func (c Class) Preview(o korrel8r.Object) string            { return Preview(o) }

// Preview returns the log body as a string.
func Preview(o korrel8r.Object) (line string) {
	if log, ok := o.(Object); ok {
		return fmt.Sprintf("%v", log.Body)
	}
	return ""
}

func (q *Query) Class() korrel8r.Class { return Class{} }
func (q *Query) String() string        { return impl.QueryString(q) }
func (q *Query) Data() string          { b, _ := json.Marshal(q.Selector); return string(b) }

func (s *Store) Domain() korrel8r.Domain                 { return Domain }
func (s *Store) StoreClasses() ([]korrel8r.Class, error) { return []korrel8r.Class{Class{}}, nil }

func (s *Store) Get(ctx context.Context, query korrel8r.Query, constraint *korrel8r.Constraint, r korrel8r.Appender) error {
	q, err := impl.TypeAssert[*Query](query)
	if err != nil {
		return err
	}
	// Get pods using the inner k8s store query.
	k8sQuery := q.k8sQuery
	pods := result.New(k8sQuery.Class())
	if err := s.s.Get(ctx, k8sQuery, constraint, pods); err != nil {
		return err
	}
	// Create query options for Pod logs
	opts := &corev1.PodLogOptions{
		Timestamps: true, // Request timestamp per log line.
		Container:  q.Container,
	}
	if start := constraint.GetStart(); !start.IsZero() {
		opts.SinceTime = &metav1.Time{Time: *constraint.Start}
	}
	if n := int64(constraint.GetLimit()); n > 0 {
		opts.TailLines = &n
	}
	errs := unique.Errors{} // Collect partial errors.
	for _, o := range pods.List() {
		pod := k8s.Wrap(o.(k8s.Object))
		// FIXME HERE expand empty container to get all logs.
		req := s.c.Pods(pod.GetNamespace()).GetLogs(pod.GetName(), opts)
		stream, err := req.Stream(context.Background())
		if errs.Add(err) {
			continue
		}
		readStream(stream, constraint, r, pod, opts.Container)
	}
	return errs.Err()
}

func readStream(stream io.ReadCloser, constraint *korrel8r.Constraint, r korrel8r.Appender, pod *unstructured.Unstructured, container string) {
	defer func() { _ = stream.Close() }()
	scanner := bufio.NewScanner(stream)
	for scanner.Scan() {
		var log Object
		timestamp, text, _ := strings.Cut(scanner.Text(), " ")
		log.Body = text
		log.Timestamp, _ = time.Parse(timestamp, time.RFC3339Nano)
		if constraint.CompareTime(log.Timestamp) != 0 {
			continue
		}
		log.Attributes = map[string]any{
			"k8s.pod.name":           pod.GetName(),
			"k8s.pod.namespace.name": pod.GetNamespace(),
			"k8s.container":          container,
			// FIXME OTEL format? labels & fields?
		}
		// FIXME other data
		r.Append(log)
	}
}
