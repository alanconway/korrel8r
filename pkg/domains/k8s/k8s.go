// Copyright: This file is part of korrel8r, released under https://github.com/korrel8r/korrel8r/blob/main/LICENSE

// Package k8s implements [Kubernetes] resources stored in a Kube API server.
//
// # Store
//
// The k8s domain automatically connects to the current cluster (as determined by kubectl),
// no additional configuration is needed.
//
//	 stores:
//		  domain: k8s
//
// [Kubernetes]: https://kubernetes.io/docs/concepts/overview/
package k8s

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	// FIXME Clean up

	"github.com/grafana/regexp"
	"github.com/korrel8r/korrel8r/pkg/korrel8r"
	"github.com/korrel8r/korrel8r/pkg/korrel8r/impl"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// NewDomain creates a Kubernetes domain using the default kube configuration.
func NewDomain() (korrel8r.Domain, error) {
	cfg, err := GetConfig()
	if err != nil {
		return nil, err
	}
	c, err := NewClient(cfg)
	if err != nil {
		return nil, err
	}
	return NewDomainWith(cfg, c), nil
}

// NewDomainWith returns a Kubernetes domain using the given configuration and client.
func NewDomainWith(cfg *rest.Config, c client.Client) *domain { return &domain{cfg: cfg, c: c} }

// Class represents a kind of kubernetes resource.
//
// The format of a class name is: "k8s:KIND.VERSION.GROUP".
// VERSION and GROUP are optional if there is no ambiguity.
//
// Examples: `k8s:Pod.v1`, `ks8:Pod`, `k8s:Deployment.v1.apps`, `k8s:Deployment.apps`, `k8s:Deployment`
type Class struct {
	schema.GroupVersionKind
	d *domain
}

// Object is a JSON map representation of a Kubernetes resource.
// Rule templates should use the JSON serialized field names, NOT the Go struct field names.
type Object map[string]any

// Unstructured wraps o in an [unstructured.Unstructured]
func Unstructured(o Object) *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: o}
}

// ObjectOf converts a [client.Object], which be structured or unstructured, to an [Object]
func ObjectOf(o client.Object) Object {
	switch o := o.(type) {
	case *unstructured.Unstructured:
		return o.Object
	default:
		b, _ := json.Marshal(o)
		var u unstructured.Unstructured
		json.Unmarshal(b, &u)
		return u.Object
	}
}

// Query struct for a Kubernetes query.
//
// Example:
//
//	k8s:Pod.v1.:{"namespace":"openshift-cluster-version","name":"cluster-version-operator-8d86bcb65-btlgn"}
type Query struct {
	// Namespace restricts the search to a namespace.
	Namespace string `json:"namespace,omitempty"`
	Name      string `json:"name,omitempty"`
	// Labels restricts the search to objects with matching label values (optional)
	Labels client.MatchingLabels `json:"labels,omitempty"`
	// Fields restricts the search to objects with matching field values (optional)
	Fields client.MatchingFields `json:"fields,omitempty"`

	class Class // class is the underlying k8s.Class object. Implied by query name prefix.
}

// Store presents the Kubernetes API server as a korrel8r.Store.
//
// The k8s domain is alread connected to a cluster, so no additional store configuration is needed.
//
//	 stores:
//		  domain: k8s
type Store struct {
	d    *domain
	base *url.URL
}

// Validate interfaces
var (
	_ korrel8r.Domain = (*domain)(nil)
	_ korrel8r.Class  = Class{}
	_ korrel8r.Object = Object(nil)
	_ korrel8r.Query  = &Query{}
)

// FIXME documentdomain implementation
type domain struct {
	c   client.Client
	cfg *rest.Config
}

func (d *domain) Name() string        { return "k8s" }
func (d *domain) String() string      { return d.Name() }
func (d *domain) Description() string { return "Resource objects in a Kubernetes API server" }
func (d *domain) Store(storeConfig any) (s korrel8r.Store, err error) {
	host := d.cfg.Host
	if host == "" {
		host = "localhost"
	}
	base, _, err := rest.DefaultServerURL(host, d.cfg.APIPath, schema.GroupVersion{}, true)
	return &Store{d: d, base: base}, err
}

var version = regexp.MustCompile(`^v[0-9]$`)

func (d *domain) Class(name string) korrel8r.Class {
	if name == "" {
		return nil
	}
	// name is one of:
	// KIND
	// KIND.VERSION[.GROUP]
	// KIND.GROUP
	kvg := strings.SplitN(name, ".", 3)
	switch len(kvg) {
	case 1:
		return d.ClassOf(schema.GroupVersionKind{Kind: kvg[0]})
	case 3:
		return d.ClassOf(schema.GroupVersionKind{Kind: kvg[0], Version: kvg[1], Group: kvg[2]})
	case 2: // Ambiguous, could be KIND.VERSION or KIND.GROUP
		if version.MatchString(kvg[1]) {
			if c := d.ClassOf(schema.GroupVersionKind{Kind: kvg[0], Version: kvg[1]}); c != nil {
				return c
			}
		}
		return d.ClassOf(schema.GroupVersionKind{Kind: kvg[0], Group: kvg[1]})
	default: // Impossible for SplitN 3
		return nil
	}
}

func (d *domain) ClassOf(gvk schema.GroupVersionKind) korrel8r.Class {
	kinds, err := d.c.RESTMapper().KindsFor(gvk.GroupVersion().WithResource(gvk.Kind))
	if err != nil {
		return nil
	}
	return Class{d: d, GroupVersionKind: kinds[0]}
}

func (d *domain) Classes() (classes []korrel8r.Class) {
	// FIXME use discovery.
	return nil
}

func (d *domain) Query(s string) (korrel8r.Query, error) {
	class, query, err := impl.UnmarshalQueryString[Query](d, s)
	if err != nil {
		return nil, err
	}
	query.class = class.(Class)
	return &query, nil
}

func (c Class) ID(o korrel8r.Object) any {
	if o, _ := o.(Object); o != nil {
		return client.ObjectKeyFromObject(Unstructured(o))
	}
	return nil
}

func (c Class) Preview(o korrel8r.Object) string {
	switch o := o.(type) {
	case *corev1.Event:
		return o.Message
	default:
		return fmt.Sprintf("%v", c.ID(o))
	}
}

func (c Class) Domain() korrel8r.Domain { return c.d }
func (c Class) Name() string {
	return fmt.Sprintf("%v.%v.%v", c.Kind, c.Version, c.Group)
}
func (c Class) String() string { return impl.ClassString(c) }
func (c Class) New() Object {
	o := &unstructured.Unstructured{}
	o.GetObjectKind().SetGroupVersionKind(c.GroupVersionKind)
	return o.Object
}
func (c Class) GVK() schema.GroupVersionKind { return c.GroupVersionKind }
func (c Class) Unmarshal(b []byte) (korrel8r.Object, error) {
	o := c.New()
	err := json.Unmarshal(b, &o)
	return o, err
}

func NewQuery(c Class, namespace, name string, labels, fields map[string]string) *Query {
	return &Query{
		Namespace: namespace,
		Name:      name,
		Labels:    labels,
		Fields:    fields,
		class:     c,
	}
}

func (q Query) Class() korrel8r.Class { return q.class }
func (q Query) Data() string          { b, _ := json.Marshal(q); return string(b) }
func (q Query) String() string        { return impl.QueryString(q) }

func (s Store) Domain() korrel8r.Domain { return s.d }
func (s Store) Client() client.Client   { return s.d.c }

func (s *Store) Get(ctx context.Context, query korrel8r.Query, c *korrel8r.Constraint, result korrel8r.Appender) (err error) {
	defer func() {
		if errors.IsNotFound(err) {
			err = nil // Finding nothing is not an error.
		}
	}()

	q, err := impl.TypeAssert[*Query](query)
	if err != nil {
		return err
	}
	appender := korrel8r.AppenderFunc(func(o korrel8r.Object) {
		// Include only objects created before or during the constraint interval.
		oo, _ := o.(Object)
		if oo != nil && c.CompareTime(Unstructured(oo).GetCreationTimestamp().Time) <= 0 {
			result.Append(o)
		}
	})
	if q.Name != "" { // Request for single object.
		return s.getObject(ctx, q, appender)
	} else {
		return s.getList(ctx, q, appender, c)
	}
}

func (s *Store) getObject(ctx context.Context, q *Query, result korrel8r.Appender) error {
	u := Unstructured(q.class.New())
	if err := s.d.c.Get(ctx, types.NamespacedName{Namespace: q.Namespace, Name: q.Name}, u); err != nil {
		return err
	}
	result.Append(ObjectOf(u))
	return nil
}

func (s *Store) getList(ctx context.Context, q *Query, result korrel8r.Appender, c *korrel8r.Constraint) (err error) {
	gvk := q.class.GVK()
	list := &unstructured.UnstructuredList{}
	list.SetGroupVersionKind(gvk.GroupVersion().WithKind(gvk.Kind + "List"))
	var opts []client.ListOption
	if q.Namespace != "" {
		opts = append(opts, client.InNamespace(q.Namespace))
	}
	if len(q.Labels) > 0 {
		opts = append(opts, q.Labels)
	}
	if len(q.Fields) > 0 {
		opts = append(opts, q.Fields)
	}
	if limit := c.GetLimit(); limit > 0 {
		opts = append(opts, client.Limit(int64(limit)))
	}
	if err := s.d.c.List(ctx, list, opts...); err != nil {
		return err
	}
	defer func() { // Handle reflect panics.
		if r := recover(); r != nil && err == nil {
			err = fmt.Errorf("invalid list object: %T", list)
		}
	}()
	for i := range list.Items {
		result.Append(ObjectOf(&list.Items[i]))
	}
	return nil
}
