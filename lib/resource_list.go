package lib

import (
	"encoding/json"
	"fmt"

	"sigs.k8s.io/kustomize/kyaml/resid"
	"sigs.k8s.io/kustomize/kyaml/yaml"
)

type Resource struct {
	// rnode yaml.RNode
	object map[string]any
}

func (r *Resource) rnode() *yaml.RNode {
	rnode, err := yaml.FromMap(r.object)
	if err != nil {
		// TODO: handle error
	}
	return rnode
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

// String implements fmt.Stringer.
func (r Resource) String() string {
	rnode := r.rnode()
	return fmt.Sprintf(
		"%s/%s %s/%s",
		rnode.GetApiVersion(), rnode.GetKind(), rnode.GetNamespace(), rnode.GetName(),
	)
}

var _ fmt.Stringer = Resource{}

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

type ResourceList struct {
	resources []*Resource
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
