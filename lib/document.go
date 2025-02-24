package lib

import (
	"encoding/json"
	"errors"

	k8sjson "sigs.k8s.io/json"
	kyaml "sigs.k8s.io/kustomize/kyaml/yaml"
)

type Document struct {
	object map[string]any
}

func NewDocumentFrom[T any](o T) Document {
	document := Document{
		object: make(map[string]any),
	}
	var err error
	switch o := any(o).(type) {
	case kyaml.RNode:
		document.object, err = o.Map()
	case *kyaml.RNode:
		document.object, err = o.Map()
	case Document:
		return o
	case map[string]any:
		document.object = o
	default:
		var b []byte
		b, err = json.Marshal(o)
		if err != nil {
			break
		}
		err = json.Unmarshal(b, &document.object)
		if err != nil {
			break
		}
	}
	if err != nil {
		FailOnError(err)
	}
	return document
}

func NewFromDocument[T any](d Document) T {
	var err error
	var oo T
	switch o := any(oo).(type) {
	case *kyaml.RNode:
		o, err = kyaml.FromMap(d.object)
		oo = any(o).(T)
	case kyaml.RNode:
		var rnode *kyaml.RNode
		rnode, err = kyaml.FromMap(d.object)
		o = *rnode
		oo = any(o).(T)
	case Document:
		o = d
		oo = any(o).(T)
	case map[string]any:
		o = d.object
		oo = any(o).(T)
	default:
		var b []byte
		b, err = json.Marshal(d.object)
		if err != nil {
			break
		}
		strictErrs, err := k8sjson.UnmarshalStrict(b, &oo, k8sjson.DisallowUnknownFields, k8sjson.DisallowDuplicateFields)
		err = errors.Join(append(strictErrs, err)...)
		if err != nil {
			break
		}
	}
	if err != nil {
		FailOnError(err)
	}
	return oo
}

func ModifyDocumentAs[T any](d *Document, fn func(*T)) {
	t := NewFromDocument[T](*d)
	fn(&t)
	d.object = NewDocumentFrom(t).object
	cleanNilAndEmpty(d.object)
	// if kubernetes object, remove status field
	if _, ok := d.object["kind"]; ok {
		delete(d.object, "status")
	}
}

func cleanNilAndEmpty(obj map[string]any) {
	for k, v := range obj {
		if v == nil {
			delete(obj, k)
			continue
		}

		switch val := v.(type) {
		case map[string]any:
			cleanNilAndEmpty(val)
		case []any:
			// Handle arrays/slices
			for i := range val {
				if m, ok := val[i].(map[string]any); ok {
					cleanNilAndEmpty(m)
				}
			}
			// Remove if array is empty
			if len(val) == 0 {
				delete(obj, k)
			}
		}
	}
}
