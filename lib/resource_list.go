package lib

import (
	"fmt"

	"github.com/ztrue/tracerr"
	"sigs.k8s.io/kustomize/kyaml/resid"
	"sigs.k8s.io/kustomize/kyaml/yaml"
)

type Resource struct {
	rnode yaml.RNode
}

func ResourceFrom[T any](o T) Resource {
	if r, ok := any(o).(Resource); ok {
		return r
	} else if r, ok := any(o).(*yaml.RNode); ok {
		return Resource{rnode: *r.Copy()}
	} else if r, ok := any(o).(yaml.RNode); ok {
		return Resource{rnode: *r.Copy()}
	}

	// TODO: marshal, unmarshal, and return Resource
	panic("for now...")
	// return Resource{RNode: yaml.MustParse(o)}
}

// String implements fmt.Stringer.
func (r Resource) String() string {
	return fmt.Sprintf(
		"%s/%s %s/%s",
		r.rnode.GetApiVersion(), r.rnode.GetKind(), r.rnode.GetNamespace(), r.rnode.GetName(),
	)
}

var _ fmt.Stringer = Resource{}

func (r *Resource) SetLabel(key, value string) {
	_, err := yaml.LabelSetter{
		Key:   key,
		Value: value,
	}.Filter(&r.rnode)
	if err != nil {
		panic(err)
	}
}

// type ResourceList interface {
// 	Append(Resource)
// 	AppendAll(ResourceList)
// 	ForEach(func(Resource))
// }

type ResourceList struct {
	resources []*Resource
	errors    []tracerr.Error
}

// func (rl *ResourceList) Error(err error) {
// 	rl.errors = append(rl.errors, tracerr.Wrap(err))
// }

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
		existingGroup, _ := resid.ParseGroupVersion(existing.rnode.GetApiVersion())
		rGroup, _ := resid.ParseGroupVersion(r.rnode.GetApiVersion())
		if existingGroup == rGroup &&
			existing.rnode.GetKind() == r.rnode.GetKind() &&
			existing.rnode.GetName() == r.rnode.GetName() &&
			existing.rnode.GetNamespace() == r.rnode.GetNamespace() {
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
	rl.errors = append(rl.errors, other.errors...)
	for _, r := range other.resources {
		if err := rl.Append(*r); err != nil {
			return err
		}
	}
	return nil
}

// func (rl *ResourceList) Resources() []*Resource {
// 	return rl.resources
// }

func (rl *ResourceList) Errors() []tracerr.Error {
	return rl.errors
}

func NewResourceList() *ResourceList {
	return &ResourceList{}
}
