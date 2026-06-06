package probe

import (
	"fmt"
	"slices"
)

func Traverse(path string, in any, opts *Options) (any, error) {
	if opts == nil {
		opts = &Options{
			Expand:  ExpandDefault,
			Missing: MissingReplace,
			Zip:     ZipStrict,
		}
	}
	opts.normalize()
	c := compile(path)

	p, err := c.Compile()
	if err != nil {
		return nil, err
	}
	res, err := p.Collect(in, opts)
	if err != nil {
		return nil, err
	}
	if a, ok := res.([]any); ok && opts.Zip != NoZip {
		return materialize(a, opts)
	}
	return res, nil
}

func materialize(arr []any, opts *Options) (any, error) {
	size, err := opts.rowCount(arr)
	if err != nil {
		return nil, err
	}
	out := make([]any, 0, size)
	for i := 0; i < size; i++ {
		var (
			tmp  = make([]any, 0, size)
			flat bool
		)
		for j := range arr {
			switch a := arr[j].(type) {
			case []any:
				if i < len(a) {
					if ok := canExpand(a[i]); ok && !flat {
						flat = true
					}
					if flat && opts.Expand == ExpandError {
						return nil, fmt.Errorf("only primitive values allowed")
					}
					if flat && opts.Expand == ExpandIgnore {
						flat = false
						tmp = append(tmp, nil)
					} else {
						tmp = append(tmp, a[i])
					}
				} else {
					tmp = append(tmp, opts.Missing)
				}
			default:
				tmp = append(tmp, a)
			}
		}
		if opts.Expand == ExpandDefault && flat {
			out = append(out, expand(tmp)...)
		} else {
			out = append(out, tmp)
		}
	}
	return out, nil
}

func expand(arr []any) []any {
	var tmp [][]any
	tmp = append(tmp, []any{})
	for i := range arr {
		a, ok := arr[i].([]any)
		if !ok {
			for j := range tmp {
				tmp[j] = append(tmp[j], arr[i])
			}
		} else {
			xs := make([][]any, 0, len(tmp)*len(a))
			for j := range a {
				for k := range tmp {
					t := slices.Clone(tmp[k])
					t = append(t, a[j])
					xs = append(xs, t)
				}
			}
			tmp = xs
		}
	}
	res := make([]any, len(tmp))
	for i := range res {
		res[i] = tmp[i]
	}
	return res
}

func canExpand(a any) bool {
	_, ok := a.([]any)
	return ok
}
