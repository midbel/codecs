package probe

func Traverse(path string, in any) ([]any, error) {
	c := compile(path)

	p, err := c.Compile()
	if err != nil {
		return nil, err
	}
	return p.Collect(in)
}

func TraverseFrom(root, path string, in any) ([]any, error) {
	starts, err := Traverse(root, in)
	if err != nil {
		return nil, err
	}
	var res []any
	for _, s := range starts {
		xs, err := Traverse(path, s)
		if err != nil {
			return nil, err
		}
		res = append(res, xs...)
	}
	return res, nil
}

func Collect(in any, paths []string) ([]any, error) {
	return nil, nil
}

func traverse(e Expr, in any) ([]any, error) {
	switch in := in.(type) {
	case []any:
		_ = in
	case map[any]string:
		_ = in
	default:
		return nil, ErrType
	}
	return nil, nil
}

func traverseArray(e Expr, in []any) ([]any, error) {
	return nil, nil
}

func traverseMap(e Expr, in map[string]any) (any, error) {
	return nil, nil
}