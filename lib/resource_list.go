package lib

import (
	"fmt"

	"sigs.k8s.io/kustomize/kyaml/resid"
	kyaml "sigs.k8s.io/kustomize/kyaml/yaml"
)

type Resource struct {
	*Document
}

func NewResource(o interface{}) Resource {
	d := NewDocumentFrom(o)
	return Resource{
		Document: &d,
	}
}

func (r *Resource) MustString() string {
	return r.rnode().MustString()
}

func (r *Resource) Kind() string {
	return r.rnode().GetKind()
}

func (r *Resource) Name() string {
	return r.rnode().GetName()
}

func (r *Resource) Namespace() string {
	return r.rnode().GetNamespace()
}

func (r *Resource) Annotation(key string) string {
	return r.rnode().GetAnnotations()[key]
}

func (r *Resource) ApplyTransformer(t Transformer) {
	rl := NewResourceList()
	rl.resources = append(rl.resources, r)
	rl.ApplyTransformer(t)
}

func (r *Resource) SetLabel(key, value string) {
	ModifyDocumentAs(r.Document, func(r *kyaml.RNode) {
		_, err := kyaml.LabelSetter{
			Key:   key,
			Value: value,
		}.Filter(r)
		if err != nil {
			panic(err)
		}
	})
}

func (r *Resource) SetAnnotation(key, value string) {
	ModifyDocumentAs(r.Document, func(r *kyaml.RNode) {
		_, err := kyaml.AnnotationSetter{
			Key:   key,
			Value: value,
		}.Filter(r)
		if err != nil {
			panic(err)
		}
	})
}

func (r *Resource) ClearAnnotation(key string) {
	ModifyDocumentAs(r.Document, func(r *kyaml.RNode) {
		_, err := kyaml.AnnotationClearer{
			Key: key,
		}.Filter(r)
		kyaml.ClearEmptyAnnotations(r)
		if err != nil {
			panic(err)
		}
	})
}

func (r *Resource) SetName(name string) {
	ModifyDocumentAs(r.Document, func(r *kyaml.RNode) {
		if err := r.SetName(name); err != nil {
			panic(err)
		}
	})
}

func (r *Resource) SetNamespace(namespace string) {
	ModifyDocumentAs(r.Document, func(r *kyaml.RNode) {
		if err := r.SetNamespace(namespace); err != nil {
			panic(err)
		}
	})
}

// String implements fmt.Stringer.
func (r Resource) String() string {
	rnode := r.rnode()
	return fmt.Sprintf(
		"%s/%s %s/%s",
		rnode.GetApiVersion(), rnode.GetKind(), rnode.GetNamespace(), rnode.GetName(),
	)
}

var _ fmt.Stringer = Resource{}

func (r *Resource) rnode() *kyaml.RNode {
	return NewFromDocument[*kyaml.RNode](*r.Document)
}

func (r *Resource) Copy() *Resource {
	rnode := NewFromDocument[*kyaml.RNode](*r.Document)
	doc := NewDocumentFrom(rnode.Copy())
	return &Resource{
		Document: &doc,
	}
}

type ResourceList struct {
	resources []*Resource
}

func (rl *ResourceList) RemoveAll(f func(r *Resource) bool) {
	var newResources []*Resource
	for _, r := range rl.resources {
		if !f(r) {
			newResources = append(newResources, r)
		}
	}
	rl.resources = newResources
}

func (rl *ResourceList) ApplyTransformer(t Transformer) {
	t(rl)
}

func (rl *ResourceList) ForEach(f func(*Resource)) {
	for i, r := range rl.resources {
		f(r)
		if err := checkDuplicates(rl.resources[:i], *r); err != nil {
			FailOnError(err)
		}
	}
}

func checkDuplicates(resources []*Resource, r Resource) error {
	for _, existing := range resources {
		existingRnode := existing.rnode()
		rnode := r.rnode()
		existingGroup, _ := resid.ParseGroupVersion(existingRnode.GetApiVersion())
		rGroup, _ := resid.ParseGroupVersion(rnode.GetApiVersion())
		if existingGroup == rGroup &&
			existingRnode.GetKind() == rnode.GetKind() &&
			existingRnode.GetName() == rnode.GetName() &&
			existingRnode.GetNamespace() == rnode.GetNamespace() {
			return fmt.Errorf(
				"resource %s already exists in list",
				r,
			)
		}
	}
	return nil
}

func (rl *ResourceList) Append(r Resource) error {
	if err := checkDuplicates(rl.resources, r); err != nil {
		return err
	}

	rl.resources = append(rl.resources, &r)
	return nil
}

func (rl *ResourceList) Absorb(other ResourceList) error {
	for _, r := range other.resources {
		if err := rl.Append(*r); err != nil {
			return err
		}
	}
	return nil
}

func NewResourceList() *ResourceList {
	return &ResourceList{}
}
