package probe

import (
	"fmt"
	"slices"
)

type Result struct {
	Sets []any
}

type Query struct {
	paths []Path
}

func Execute(path string, in any, opts *Options) (*Result, error) {
	if opts == nil {
		opts = &Options{
			Expand:  ExpandDefault,
			Missing: MissingReplace,
			Zip:     ZipStrict,
		}
	}
	opts.normalize()
	c := compile(path)

	q, err := c.Compile()
	if err != nil {
		return nil, err
	}
	return q.Execute(in, opts)
}

func (q *Query) Execute(in any, opts *Options) (*Result, error) {
	var result Result
	for _, a := range q.paths {
		for res, err := range a.All(in) {
			if err != nil {
				return nil, err
			}
			ds, err := q.prepare(res, opts)
			if err != nil {
				return nil, err
			}
			result.Sets = append(result.Sets, ds)
		}
	}
	return &result, nil
}

func (q *Query) prepare(res any, opts *Options) (any, error) {
	res, err := normalize(res, opts)
	if err != nil {
		return nil, err
	}
	if a, ok := res.([]any); ok && opts.Zip != NoZip {
		res, err = materialize(a, opts)
		if err != nil {
			return nil, err
		}
	}
	return clean(res), nil
}

func clean(in any) any {
	switch in := in.(type) {
	case discard:
		return nil
	case []any:
		tmp := make([]any, 0, len(in))
		for i := range in {
			if isDiscard(in[i]) {
				continue
			}
			tmp = append(tmp, clean(in[i]))
		}
		return tmp
	case map[string]any:
		tmp := make(map[string]any)
		for k := range in {
			if isDiscard(in[k]) {
				continue
			}
			in[k] = clean(in[k])
		}
		return tmp
	default:
		return in
	}
}

func normalize(in any, opts *Options) (any, error) {
	switch x := in.(type) {
	case missing:
		if opts.Missing == MissingReplace {
			in = opts.MissingValue
		} else if opts.Missing == MissingNull {
			in = nil
		} else if opts.Missing == MissingIgnore {
			// pass
		} else if opts.Missing == MissingError {
			return nil, fmt.Errorf("missing value")
		} else {
			return nil, fmt.Errorf("missing value can not be handled")
		}
	case []any:
		tmp := make([]any, 0, len(x))
		for i := range x {
			v, err := normalize(x[i], opts)
			if err != nil {
				return nil, err
			}
			if opts.Missing == MissingIgnore && isMissing(v) {
				continue
			}
			tmp = append(tmp, v)
		}
		in = tmp
	case map[string]any:
		tmp := make(map[string]any)
		for k := range x {
			v, err := normalize(x[k], opts)
			if err != nil {
				return nil, err
			}
			tmp[k] = v
		}
		in = tmp
	default:
	}
	return in, nil
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
