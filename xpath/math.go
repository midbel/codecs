package xpath

import (
	"fmt"
	"math"
	"time"

	"github.com/midbel/codecs/xml"
)

type BinaryFunc func(Sequence, Sequence) (Sequence, error)

var binaryOp = map[rune]BinaryFunc{
	opAdd:    doAdd,
	opSub:    doSub,
	opMul:    doMul,
	opDiv:    doDiv,
	opMod:    doMod,
	opConcat: doConcat,
	opAnd:    doAnd,
	opOr:     doOr,
	opBefore: doBefore,
	opAfter:  doAfter,
	opEq:     doEqual,
	opNe:     doNotEqual,
	opLt:     doLesser,
	opLe:     doLessEq,
	opGt:     doGreater,
	opGe:     doGreatEq,
	opValEq:  doOups,
	opValNe:  doOups,
	opValLt:  doOups,
	opValLe:  doOups,
	opValGt:  doOups,
	opValGe:  doOups,
}

func doOups(_, _ Sequence) (Sequence, error) {
	return nil, fmt.Errorf("not yet supported")
}

func doAdd(left, right Sequence) (Sequence, error) {
	return apply(left, right, func(left, right float64) (float64, error) {
		return left + right, nil
	})
}

func doSub(left, right Sequence) (Sequence, error) {
	return apply(left, right, func(left, right float64) (float64, error) {
		return left - right, nil
	})
}

func doMul(left, right Sequence) (Sequence, error) {
	return apply(left, right, func(left, right float64) (float64, error) {
		return left * right, nil
	})
}

func doDiv(left, right Sequence) (Sequence, error) {
	return apply(left, right, func(left, right float64) (float64, error) {
		if right == 0 {
			return 0, ErrZero
		}
		return left / right, nil
	})
}

func doMod(left, right Sequence) (Sequence, error) {
	return apply(left, right, func(left, right float64) (float64, error) {
		if right == 0 {
			return 0, ErrZero
		}
		return math.Mod(left, right), nil
	})
}

func doConcat(left, right Sequence) (Sequence, error) {
	var str1, str2 string
	if !left.Empty() {
		str1, _ = toString(left[0].Value())
	}
	if !right.Empty() {
		str2, _ = toString(right[0].Value())
	}
	return Singleton(str1 + str2), nil
}

func doAnd(left, right Sequence) (Sequence, error) {
	ok := left.True() && right.True()
	return Singleton(ok), nil
}

func doOr(left, right Sequence) (Sequence, error) {
	ok := left.True() || right.True()
	return Singleton(ok), nil
}

func doBefore(left, right Sequence) (Sequence, error) {
	if left.Empty() || right.Empty() {
		return Singleton(false), nil
	}
	ok := xml.Before(left[0].Node(), right[0].Node())
	return Singleton(ok), nil
}

func doAfter(left, right Sequence) (Sequence, error) {
	if left.Empty() || right.Empty() {
		return Singleton(false), nil
	}
	ok := xml.After(left[0].Node(), right[0].Node())
	return Singleton(ok), nil
}

func doEqual(left, right Sequence) (Sequence, error) {
	res, err := isEqual(left, right)
	return Singleton(res), err
}

func doNotEqual(left, right Sequence) (Sequence, error) {
	res, err := isEqual(left, right)
	if err != nil {
		return nil, err
	}
	return Singleton(!res), nil
}

func doLesser(left, right Sequence) (Sequence, error) {
	res, err := isLess(left, right)
	if err != nil {
		return nil, err
	}
	return Singleton(res), nil
}

func doLessEq(left, right Sequence) (Sequence, error) {
	ok, err := isEqual(left, right)
	if err != nil {
		return nil, err
	}
	if ok {
		return Singleton(ok), nil
	}
	ok, err = isLess(left, right)
	if err != nil {
		return nil, err
	}
	return Singleton(ok), nil
}

func doGreater(left, right Sequence) (Sequence, error) {
	ok, err := isEqual(left, right)
	if err != nil {
		return nil, err
	}
	if ok {
		return Singleton(false), nil
	}
	ok, err = isLess(left, right)
	if err != nil {
		return nil, err
	}
	return Singleton(!ok), nil
}

func doGreatEq(left, right Sequence) (Sequence, error) {
	ok, err := isEqual(left, right)
	if err != nil {
		return nil, err
	}
	if ok {
		return Singleton(ok), nil
	}
	ok, err = isLess(left, right)
	if err != nil {
		return nil, err
	}
	return Singleton(!ok), nil
}

func apply(left, right Sequence, do func(left, right float64) (float64, error)) (Sequence, error) {
	if left.Empty() || right.Empty() {
		return Singleton(math.NaN()), nil
	}
	var res Sequence
	for i := range left {
		x, err := toFloat(left[i].Value())
		if err != nil {
			return nil, err
		}
		for j := range right {
			y, err := toFloat(right[j].Value())
			if err != nil {
				return nil, err
			}
			v, err := do(x, y)
			if err != nil {
				return nil, err
			}
			res.Append(NewLiteralItem(v))
		}
	}
	return res, nil
}

func compareItems(left, right Sequence, cmp func(left, right Item) (bool, error)) (bool, error) {
	if left.Empty() || right.Empty() {
		return false, nil
	}
	for i := range left {
		for j := range right {
			ok, err := cmp(left[i], right[j])
			if ok || err != nil {
				return ok, err
			}
		}
	}
	return false, nil
}

func isLess(left, right Sequence) (bool, error) {
	return compareItems(left, right, func(left, right Item) (bool, error) {
		switch x := left.Value().(type) {
		case float64:
			y, err := toFloat(right.Value())
			return x < y, err
		case string:
			y, err := toString(right.Value())
			return x < y, err
		case time.Time:
			y, err := toTime(right.Value())
			return x.Before(y), err
		default:
			return false, ErrType
		}
	})

}

func isEqual(left, right Sequence) (bool, error) {
	return compareItems(left, right, func(left, right Item) (bool, error) {
		switch x := left.Value().(type) {
		case float64:
			y, err := toFloat(right.Value())
			return nearlyEqual(x, y), err
		case string:
			y, err := toString(right.Value())
			return x == y, err
		case bool:
			return toBool(right.Value())
		case time.Time:
			y, err := toTime(right.Value())
			return x.Equal(y), err
		default:
			return false, ErrType
		}
	})
}

func nearlyEqual(left, right float64) bool {
	if left == right {
		return true
	}
	return math.Abs(left-right) < 0.000001
}

func lowestValue[T string | float64](items []T) T {
	var res T
	for i := range items {
		if i == 0 {
			res = items[i]
			continue
		}
		res = min(items[i], res)
	}
	return res
}

func greatestValue[T string | float64](items []T) T {
	var res T
	for i := range items {
		if i == 0 {
			res = items[i]
			continue
		}
		res = max(items[i], res)
	}
	return res
}
