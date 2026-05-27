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
