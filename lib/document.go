package lib

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/GoogleContainerTools/kpt-functions-catalog/functions/go/search-replace/searchreplace"
	k8sjson "sigs.k8s.io/json"
	"sigs.k8s.io/kustomize/kyaml/utils"
	kyaml "sigs.k8s.io/kustomize/kyaml/yaml"
)

type Document struct {
	object map[string]any
}

func (d *Document) RegexReplaceValues(value, replacement string) {
	filter := searchreplace.SearchReplace{
		ByValueRegex: value,
		PutValue:     replacement,
	}

	ModifyDocumentAs(d, func(rnode *kyaml.RNode) {
		_, err := filter.Filter([]*kyaml.RNode{rnode})
		FailOnError(err)
	})
}

func (d *Document) ReplaceValues(value, replacement string) {
	filter := searchreplace.SearchReplace{
		ByValue:  value,
		PutValue: replacement,
	}

	ModifyDocumentAs(d, func(rnode *kyaml.RNode) {
		_, err := filter.Filter([]*kyaml.RNode{rnode})
		FailOnError(err)
	})
}

func (d *Document) ReplacePaths(path string, v any) {
	ModifyDocumentAs(d, func(rnode *kyaml.RNode) {
		bytes, err := kyaml.Marshal(v)
		if err != nil {
			FailOnError(fmt.Errorf("value cannot be serialized as yaml: %w", err))
		}

		value, err := kyaml.Parse(string(bytes))
		if err != nil {
			panic(err)
		}

		targetFieldList, err := rnode.Pipe(&kyaml.PathMatcher{Path: utils.SmarterPathSplitter(path, "."), Create: value.YNode().Kind})
		if err != nil {
			FailOnError(fmt.Errorf("failed to find finds: %w", err))
		}

		targetFields, err := targetFieldList.Elements()
		if err != nil {
			panic(err)
		}
		if len(targetFields) == 0 {
			FailOnError(fmt.Errorf("failed to match any fields"))
		}

		for _, targetField := range targetFields {
			if targetField.YNode().Kind == kyaml.ScalarNode {
				// For scalar, only copy the value (leave any type intact to auto-convert int->string or string->int)
				targetField.YNode().Value = value.YNode().Value
			} else {
				targetField.SetYNode(value.YNode())
			}
		}
	})
}

func NewDocument[T any](o T) Document {
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
	d.object = NewDocument(t).object
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
