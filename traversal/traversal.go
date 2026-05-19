package traversal

type Step struct {
	Field string
	Cast  string
}

func Collect(in any, paths []string) ([]any, error) {
	st := make([]Step, 0, len(paths))
	for _, p := range paths {
		s := Step{
			Field: p,
		}
		st = append(st, s)
	}
	return collect(in, st)
}

func collect(in any, paths []Step) ([]any, error) {
	switch in := in.(type) {
	case []any:
		return traverseArray(in, paths)
	case map[string]any:
		return traverseObject(in, paths)
	default:
		return nil, nil
	}
}

func traverseArray(in []any, paths []Step) ([]any, error) {
	return nil, nil
}

func traverseObject(in map[string]any, paths []Step) ([]any, error) {
	return nil, nil
}
