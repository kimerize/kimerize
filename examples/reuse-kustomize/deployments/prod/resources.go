//go:generate go run ./
package main

import (
	"github.com/kimerize/kimerize/examples/reuse-kustomize/multibases"
	. "github.com/kimerize/kimerize/lib"
)

func main() {
	FailOnError(
		KustomizePublisher().Publish(
			BuildKustomizeLayer("all", EmbedFilesysBuilder(multibases.FS)),
		),
	)
}
