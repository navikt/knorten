package api

import "k8s.io/utils/strings/slices"

func removeEmptySliceElements(sliceElements []string) []string {
	return slices.Filter(nil, sliceElements, func(s string) bool {
		return s != ""
	})
}
