package image

type ListFilter func(*listFilters)

type listFilters struct {
	references []string
}

func ByReference(reference string) ListFilter {
	return func(filters *listFilters) {
		filters.references = append(filters.references, reference)
	}
}
