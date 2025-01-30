package main

import (
	"github.com/kimerize/kimerize/examples/reuse-kustomize/multibases"
	. "github.com/kimerize/kimerize/lib"
)

var Resources ResourceList = BuildKustomizeLayer("all", EmbedFilesysBuilder(multibases.FS))

var Publisher PackagePublisher = KustomizePublisher()
