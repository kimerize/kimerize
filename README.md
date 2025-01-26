# kimerize prototype

## Goals

Inspired by Kustomize, Tanka, CUE.

- provide a framework in a full-feature programming language to generate Kubernetes manifests
    - avoids custom DSL
    - enables usual debugging workflows
- Enable easy overrides for a specific configuration variant

# High level idea

Each package/Overlay is implemented similar to kustomize layers - it uses some resources from other layers, and runs additional transformations on top.

Overlay is a Generator type. Each Overlay has a Transformer that consists of lower level Generators (equivalent to layer or simple resource entry in kustomize), and arbitrary configuration options that are passed to transform function. Transformer with config enables similar style of overrides as in Tanka.

To generate an Overlay:

1. Overlay finds all Generators in the Transformer struct and generates them
2. Runs Transform function on those resources with the Config


This kind of structure enables easy overrides of parameters in base layer. See [prod overrides](example/deployments/prod-eu1/resources.go) for example.

## Demo

`kimerize` cmd will find all `main` packages that contain a `Resoruces` symbol of `Generator` type.

To generate the manifests, run:

```shell
make
```

## Future work
- hermetic runs
- global transformers (e.g. run validators or transformers at last stage like https://github.com/google/k8s-digester)
- add utility functions
    - filtering
    - casting to specific types to Transform the objects with even more ease
