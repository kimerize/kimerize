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
	d := NewDocument(o)
	return Resource{
		Document: &d,
	}
}

func (r *Resource) ApiVersion() (string, bool) {
	v, ok := r.object["apiVersion"]
	if !ok {
		FailOnError(fmt.Errorf("apiVersion not found"))
	}
	s, ok := v.(string)
	if !ok {
		FailOnError(fmt.Errorf("apiVersion is not a string"))
	}
	return s, true
}

func (r *Resource) Kind() (string, bool) {
	v, ok := r.object["kind"]
	if !ok {
		return "", false
	}
	s, ok := v.(string)
	if !ok {
		FailOnError(fmt.Errorf("kind is not a string"))
	}
	return s, true
}

func (r *Resource) metadata() map[string]interface{} {
	v, ok := r.object["metadata"]
	if !ok {
		return map[string]interface{}{}
	}
	m, ok := v.(map[string]interface{})
	if !ok {
		FailOnError(fmt.Errorf("metadata not an object"))
	}
	return m

}

func (r *Resource) Name() (string, bool) {
	v, ok := r.metadata()["name"]
	if !ok {
		return "", false
	}
	s, ok := v.(string)
	if !ok {
		FailOnError(fmt.Errorf("name is not a string"))
	}
	return s, true
}

func (r *Resource) Namespace() (string, bool) {
	v, ok := r.metadata()["namespace"]
	if !ok {
		return "", false
	}
	s, ok := v.(string)
	if !ok {
		FailOnError(fmt.Errorf("namespace is not a string"))
	}
	return s, true
}

func (r *Resource) Annotation(key string) (string, bool) {
	av, ok := r.metadata()["annotations"]
	if !ok {
		return "", false
	}
	annotations, ok := av.(map[string]interface{})
	if !ok {
		FailOnError(fmt.Errorf("annotations not an object"))
	}
	v, ok := annotations[key]
	if !ok {
		return "", false
	}
	s, ok := v.(string)
	if !ok {
		FailOnError(fmt.Errorf("annotation is not a string"))
	}
	return s, true
}

func (r *Resource) Label(key string) (string, bool) {
	av, ok := r.metadata()["labels"]
	if !ok {
		return "", false
	}
	labels, ok := av.(map[string]interface{})
	if !ok {
		FailOnError(fmt.Errorf("labels not an object"))
	}
	v, ok := labels[key]
	if !ok {
		return "", false
	}
	s, ok := v.(string)
	if !ok {
		FailOnError(fmt.Errorf("label is not a string"))
	}
	return s, true
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
	apiVersion, _ := r.ApiVersion()
	kind, _ := r.Kind()
	namespace, _ := r.Namespace()
	name, _ := r.Name()
	return fmt.Sprintf(
		"%s/%s %s/%s",
		apiVersion, kind, namespace, name,
	)
}

var _ fmt.Stringer = Resource{}

// func (r *Resource) rnode() *kyaml.RNode {
// 	return NewFromDocument[*kyaml.RNode](*r.Document)
// }

func (r *Resource) Copy() *Resource {
	rnode := NewFromDocument[*kyaml.RNode](*r.Document)
	doc := NewDocument(rnode.Copy())
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
		existingApiVersion, _ := existing.ApiVersion()
		rApiVersion, _ := r.ApiVersion()
		existingGroup, _ := resid.ParseGroupVersion(existingApiVersion)
		rGroup, _ := resid.ParseGroupVersion(rApiVersion)
		existingKind, _ := existing.Kind()
		rKind, _ := r.Kind()
		existingName, _ := existing.Name()
		rName, _ := r.Name()
		existingNamespace, _ := existing.Namespace()
		rNamespace, _ := r.Namespace()
		if existingGroup == rGroup &&
			existingKind == rKind &&
			existingName == rName &&
			existingNamespace == rNamespace {
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
