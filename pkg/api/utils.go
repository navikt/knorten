package api

import (
	"slices"
)

func removeEmptySliceElements(sliceElements []string) []string {
	return slices.DeleteFunc(sliceElements, func(s string) bool {
		return s == ""
	})
}
