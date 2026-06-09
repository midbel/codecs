package probe

import (
	"fmt"
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

func ParseMissingMode(str string) (MissingMode, error) {
	var mode MissingMode
	switch str {
	case "replace":
		mode = MissingReplace
	case "null":
		mode = MissingNull
	case "ignore":
		mode = MissingIgnore
	case "error":
		mode = MissingError
	default:
		return mode, fmt.Errorf("unsupported missing mode given: %s", str)
	}
	return mode, nil
}

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
