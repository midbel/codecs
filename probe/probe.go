package probe

import "fmt"

func Traverse(path string, in any) (any, error) {
	c := compile(path)

	p, err := c.Compile()
	if err != nil {
		return nil, err
	}
	return p.Collect(in)
}

func TraverseFrom(root, path string, in any) (any, error) {
	starts, err := Traverse(root, in)
	if err != nil {
		return nil, err
	}
	return Traverse(path, starts)
}

func Collect(in any, paths []string) ([]any, error) {
	return nil, nil
}

func traverse(e Expr, in any) ([]any, error) {
	if e, ok := e.(literal); ok {
		return []any{e.value}, nil
	}
	switch in := in.(type) {
	case []any:
		return traverseArray(e, in)
	case map[string]any:
		return traverseMap(e, in)
	default:
		return nil, ErrType
	}
}

func traverseArray(e Expr, in []any) ([]any, error) {
	var result []any
	for i := range in {
		tmp, err := traverse(e, in[i])
		if err != nil {
			return nil, err
		}
		result = append(result, tmp...)
	}
	return result, nil
}

func traverseMap(e Expr, in map[string]any) ([]any, error) {
	if e == nil {
		return nil, ErrEnd
	}
	x, ok := e.(field)
	if !ok {
		return nil, nil
	}
	p, ok := in[x.Name]
	if !ok {
		return nil, fmt.Errorf("%s: %w", x.Name, ErrProp)
	}
	if x.Next == nil {
		return []any{p}, nil
	}
	return traverse(x.Next, p)
}
