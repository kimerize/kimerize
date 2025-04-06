package utils

import (
	. "github.com/kimerize/kimerize/lib"
)

func CompanyTransforms() []Transformer {
	return []Transformer{
		DigestImagesTransformer(),
	}
}
