package lib

import (
	"context"
	"reflect"

	"github.com/go-logr/logr"
	"github.com/google/k8s-digester/pkg/resolve"
	"sigs.k8s.io/kustomize/kyaml/yaml"
)

type Overlay interface {
	Transform(items *ResourceList)
	SetDefaults()
}

type overlay[T any, P interface {
	*T
	Overlay
}] struct {
	config P
}

func setDefaults(o Overlay) {
	v := reflect.ValueOf(o)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	if v.Kind() != reflect.Struct {
		panic("Overlay config must be a struct")
	}

	for i := 0; i < v.NumField(); i++ {
		field := v.Field(i)
		v := field.Addr().Interface()
		if g, ok := v.(Overlay); ok {
			setDefaults(g)
		}
	}
	o.SetDefaults()
}

func BuildOverlayWithOverrides[T any, P interface {
	*T
	Overlay
}](override func(*T)) ResourceList {
	t := *new(T)
	var p P = &t
	setDefaults(p)
	if override != nil {
		override(&t)
	}
	overlay := overlay[T, P]{
		config: p,
	}
	return overlay.Generate()
}

func BuildOverlayWithoutOverrides[T any, P interface {
	*T
	Overlay
}]() ResourceList {
	return BuildOverlayWithOverrides[T, P](nil)
}

func Aggregate(resources ...ResourceList) ResourceList {
	result := NewResourceList()
	for _, r := range resources {
		result.Absorb(r)
	}
	return *result
}

type Transformer func(*ResourceList)

func Transform(resources ResourceList, transformers ...Transformer) ResourceList {
	for _, t := range transformers {
		t(&resources)
	}
	return resources
}

type buildOverlayOverrides[T any, P interface {
	*T
	Overlay
}] struct {
	override func(*T)
}

func WithOverrides[T any, P interface {
	*T
	Overlay
}](override func(*T)) buildOverlayOverrides[T, P] {
	return buildOverlayOverrides[T, P]{
		override: override,
	}
}

func WithoutOverrides[T any, P interface {
	*T
	Overlay
}]() buildOverlayOverrides[T, P] {
	return buildOverlayOverrides[T, P]{}
}

type transform struct {
	transform func(*ResourceList)
}

type Tran interface {
}

func WithTransform(f func(*ResourceList)) transform {
	return transform{
		transform: f,
	}
}

type dummyOverlayConfig struct {
	Kind string
}

func (dummyOverlayConfig) Transform(items *ResourceList) {}

func (d *dummyOverlayConfig) SetDefaults() {
	d.Kind = "dummy"
}

var _ Overlay = &dummyOverlayConfig{}

func (t overlay[T, P]) Generate() ResourceList {
	return generate(t.config)
}

func generate(o Overlay) ResourceList {
	v := reflect.ValueOf(o)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	if v.Kind() != reflect.Struct {
		panic("Overlay config must be a struct")
	}

	result := NewResourceList()
	for i := 0; i < v.NumField(); i++ {
		field := v.Field(i)
		v := field.Addr().Interface()
		if g, ok := v.(Overlay); ok {
			result.Absorb(generate(g))
		}
	}
	o.Transform(result)
	return *result
}

func DigestImages(rl *ResourceList) {
	rl.ForEach(func(r *Resource) {
		ModifyAs(r, func(r *yaml.RNode) {
			resolve.ImageTags(context.TODO(), logr.Discard(), nil, r, nil)
		})
	})
}
