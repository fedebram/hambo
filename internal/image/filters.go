package image

type ListFilter func(*listFilters)

type listFilters struct {
	names []string
}

func ByName(name string) ListFilter {
	return func(filters *listFilters) {
		filters.names = append(filters.names, name)
	}
}
