// Copyright: This file is part of korrel8r, released under https://github.com/korrel8r/korrel8r/blob/main/LICENSE

package k8s

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
)

// FIXME

func New[T any, PT interface {
	client.Object
	*T
}](namespace, name string) PT {
	o := PT(new(T))
	gvk, err := apiutil.GVKForObject(o, builtIn) // FIXME
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
