package lib

import (
	"encoding/json"
	"errors"
	"fmt"

	k8sjson "sigs.k8s.io/json"
	"sigs.k8s.io/kustomize/kyaml/resid"
	"sigs.k8s.io/kustomize/kyaml/yaml"
)

type Resource struct {
	object map[string]any
}

func FromMap(m map[string]any) Resource {
	return Resource{
		object: m,
	}
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

func (r *Resource) ApplyTransformer(t Transformer) {
	rl := NewResourceList()
	rl.resources = append(rl.resources, r)
	rl.ApplyTransformer(t)
}

func (r *Resource) SetLabel(key, value string) {
	ModifyAs(r, func(r *yaml.RNode) {
		_, err := yaml.LabelSetter{
			Key:   key,
			Value: value,
		}.Filter(r)
		if err != nil {
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

func (r *Resource) rnode() *yaml.RNode {
	rnode, err := yaml.FromMap(r.object)
	if err != nil {
		// TODO: handle error
	}
	return rnode
}

func (r *Resource) Copy() *Resource {
	object, err := r.rnode().Map()
	if err != nil {
		panic(err)
	}
	return &Resource{
		object: object,
	}
}

func ResourceFrom[T any](o T) Resource {
	resource := Resource{
		object: make(map[string]any),
	}
	var err error
	switch o := any(o).(type) {
	case yaml.RNode:
		resource.object, err = o.Map()
	case *yaml.RNode:
		resource.object, err = o.Map()
	case Resource:
		return o
	default:
		var b []byte
		b, err = json.Marshal(o)
		if err != nil {
			break
		}
		err = json.Unmarshal(b, &resource.object)
		if err != nil {
			break
		}
	}
	if err != nil {
		// TODO: handle error
	}
	return resource
}

func ModifyAs[T any](r *Resource, fn func(*T)) {
	switch any(*new(T)).(type) {
	case yaml.RNode:
		rnode, err := yaml.FromMap(r.object)
		if err != nil {
			panic(err)
		}

		fn(any(rnode).(*T))

		object, err := rnode.Map()
		if err != nil {
			panic(err)
		}
		r.object = object
	default:
		var t T

		b, err := json.Marshal(r.object)
		if err != nil {
			panic(err)
		}
		strictErrs, err := k8sjson.UnmarshalStrict(b, &t, k8sjson.DisallowUnknownFields, k8sjson.DisallowDuplicateFields)
		if err := errors.Join(append(strictErrs, err)...); err != nil {
			FailOnError(err)
		}

		fn(&t)

		b, err = json.Marshal(t)
		if err != nil {
			panic(err)
		}
		newObject := make(map[string]any)
		err = json.Unmarshal(b, &newObject)
		if err != nil {
			panic(err)
		}
		r.object = newObject
	}
}

type ResourceList struct {
	resources []*Resource
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
