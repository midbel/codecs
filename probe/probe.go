package probe

import (
	"fmt"
	"slices"
)

type ZipMode int8

const (
	NoZip ZipMode = 1 << iota
	ZipShort
	ZipLongest
	ZipStrict
)

func ParseZipMode(str string) (ZipMode, error) {
	var mode ZipMode
	switch str {
	case "", "short", "default":
		mode = ZipShort
	case "longest":
		mode = ZipLongest
	case "strict":
		mode = ZipStrict
	default:
		return mode, fmt.Errorf("unsupported zip mode given: %s", str)
	}
	return mode, nil
}

type ExpandMode int8

const (
	ExpandDefault ExpandMode = 1 << iota
	ExpandIgnore
	ExpandError
)

type MissingMode int8

const (
	MissingReplace MissingMode = 1 << iota
	MissingNull
	MissingIgnore
	MissingError
)

func ParseExpandMode(str string) (ExpandMode, error) {
	var mode ExpandMode
	switch str {
	case "", "default":
		mode = ExpandDefault
	case "ignore":
		mode = ExpandIgnore
	case "strict":
		mode = ExpandError
	default:
		return mode, fmt.Errorf("unsupported expand mode given: %s", str)
	}
	return mode, nil
}

type Options struct {
	Zip          ZipMode
	Expand       ExpandMode
	Missing      MissingMode
	MissingValue any
}

func (o *Options) normalize() {
	if o.Zip == 0 {
		o.Zip = ZipStrict
	}
	if o.Expand == 0 {
		o.Expand = ExpandDefault
	}
	if o.Missing == 0 {
		o.Missing = MissingReplace
	}
}

func (o *Options) rowCount(in []any) (int, error) {
	var (
		res int
		err error
	)
	switch o.Zip {
	case ZipShort:
		res = o.minSize(in)
	case ZipLongest:
		res = o.maxSize(in)
	case ZipStrict:
		res, err = o.strictSize(in)
	default:
		err = fmt.Errorf("no zip")
	}
	if err != nil {
		return res, err
	}
	if res == 0 {
		res++
	}
	return res, nil
}

func (o *Options) minSize(arr []any) int {
	var (
		size int
		set  bool
	)
	for _, a := range arr {
		if a, ok := a.([]any); ok {
			if !set {
				size = len(a)
				set = true
			}
			size = min(size, len(a))
		}
	}
	return size
}

func (o *Options) maxSize(arr []any) int {
	var size int
	for _, a := range arr {
		if a, ok := a.([]any); ok {
			size = max(size, len(a))
		}
	}
	return size
}

func (o *Options) strictSize(arr []any) (int, error) {
	var (
		size int
		set  bool
	)
	for _, a := range arr {
		if a, ok := a.([]any); ok {
			if !set {
				set = true
				size = len(a)
			}
			if size != len(a) {
				return 0, fmt.Errorf("size mismatched!")
			}
		}
	}
	return size, nil
}

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
