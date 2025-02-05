package casing

import (
	"iter"
	"unicode"
	"unicode/utf8"
)

type CaseType int8

const (
	DefaultCase CaseType = -(1 << iota)
	SnakeCase
	KebabCase
	CamelCase
	PascalCase
)

func To(to CaseType, str string) string {
	switch to {
	case SnakeCase:
		str = ToSnake(str)
	case KebabCase:
		str = ToKebab(str)
	case CamelCase:
		str = ToCamel(str)
	case PascalCase:
		str = ToPascal(str)
	default:
	}
	return str
}

func ToSnake(str string) string {
	var (
		chars []rune
		last  rune
	)
	for r := range iterRunes(str) {
		if r == space || r == hyphen {
			chars = append(chars, underscore)
		} else if unicode.IsUpper(r) && last != 0 {
			if !isSep(last) && !unicode.IsUpper(last) {
				chars = append(chars, underscore)
			}
			chars = append(chars, unicode.To(unicode.LowerCase, r))
		} else {
			chars = append(chars, unicode.To(unicode.LowerCase, r))
		}
		last = r
	}
	if z := len(chars); z > 0 && isSep(last) {
		chars = chars[:z-1]
	}
	return string(chars)
}

func ToKebab(str string) string {
	var (
		chars []rune
		last  rune
	)
	for r := range iterRunes(str) {
		if r == space || r == underscore {
			chars = append(chars, hyphen)
		} else if unicode.IsUpper(r) && last != 0 {
			if !isSep(last) && !unicode.IsUpper(last) {
				chars = append(chars, hyphen)
			}
			chars = append(chars, unicode.To(unicode.LowerCase, r))
		} else {
			chars = append(chars, unicode.To(unicode.LowerCase, r))
		}
		last = r
	}
	if z := len(chars); z > 0 && isSep(last) {
		chars = chars[:z-1]
	}
	return string(chars)
}

func ToPascal(str string) string {
	var chars []rune

	next, stop := iter.Pull(iterRunes(str))
	defer stop()
	for i := 0; ; i++ {
		r, ok := next()
		if !ok {
			break
		}

		if i == 0 && !unicode.IsUpper(r) {
			chars = append(chars, unicode.To(unicode.UpperCase, r))
		} else if isSep(r) {
			r, ok := next()
			if !ok {
				break
			}
			chars = append(chars, unicode.To(unicode.UpperCase, r))
		} else {
			chars = append(chars, r)
		}
	}
	return string(chars)
}

func ToCamel(str string) string {
	var chars []rune

	next, stop := iter.Pull(iterRunes(str))
	defer stop()
	for {
		r, ok := next()
		if !ok {
			break
		}

		if isSep(r) {
			r, ok := next()
			if !ok {
				break
			}
			chars = append(chars, unicode.To(unicode.UpperCase, r))
		} else {
			chars = append(chars, r)
		}
	}
	return string(chars)
}

const (
	hyphen     = '-'
	space      = ' '
	underscore = '_'
)

func isSep(r rune) bool {
	return r == hyphen || r == underscore || r == space
}

func iterRunes(str string) iter.Seq[rune] {
	keep := func(r rune) bool {
		return unicode.IsLetter(r) || unicode.IsDigit(r) || isSep(r)
	}
	skip := func(str string) int {
		var offset int
		for offset < len(str) {
			r, z := utf8.DecodeRuneInString(str[offset:])
			if !isSep(r) {
				break
			}
			offset += z
		}
		return offset
	}
	fn := func(yield func(rune) bool) {
		var (
			last   rune
			offset = skip(str)
		)
		for offset < len(str) {
			r, z := utf8.DecodeRuneInString(str[offset:])
			offset += z

			if !keep(r) {
				continue
			} else if isSep(last) && isSep(r) {
				offset += skip(str[offset:])
				continue
			}

			if !yield(r) {
				break
			}
			last = r
		}
	}
	return fn
}
