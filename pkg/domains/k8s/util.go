// Copyright: This file is part of korrel8r, released under https://github.com/korrel8r/korrel8r/blob/main/LICENSE

package k8s

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
)

// FIXME Remove?

func New[T any, PT interface {
	client.Object
	*T
}](namespace, name string) PT {
	o := PT(new(T))
	gvk, err := apiutil.GVKForObject(o, scheme.Scheme)
	if err != nil {
		panic(err)
	}
	o.GetObjectKind().SetGroupVersionKind(gvk)
	o.SetNamespace(namespace)
	o.SetName(name)
	return o
}

func EventFor(o client.Object, name string) *corev1.Event {
	gvk := o.GetObjectKind().GroupVersionKind()
	e := New[corev1.Event](name, o.GetNamespace())
	e.InvolvedObject = corev1.ObjectReference{
		Kind:       gvk.Kind,
		Namespace:  o.GetNamespace(),
		Name:       o.GetName(),
		APIVersion: gvk.GroupVersion().String(),
	}
	return e
}

func Create(c client.Client, objs ...client.Object) error {
	for _, o := range objs {
		if err := c.Create(context.Background(), o); err != nil {
			return err
		}
	}
	return nil
}

// GroupVersionKind returns the GVK of o, which must be a pointer to a typed API resource struct.
// Returns empty if o is not a known resource type.
func GVK(o client.Object) schema.GroupVersionKind {
	if gvks, _, err := scheme.Scheme.ObjectKinds(o); err == nil {
		return gvks[0]
	}
	return schema.GroupVersionKind{}
}
