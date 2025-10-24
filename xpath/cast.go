package xpath

import (
	"errors"
	"strconv"
	"time"

	"github.com/midbel/codecs/xml"
)

type XdmType interface {
	Name() xml.QName
	InstanceOf(Expr) bool
	Cast(Expr) Expr
	Castable(Expr) bool
}

var (
	xsUntyped = &untypedType{}
	xsAnyType = &anyType{
		parent: xsUntyped,
	}
	xsAny = &anyAtomicType{
		parent: xsAnyType,
	}
	xsString = &stringType{
		parent: xsAnyType,
	}
	xsBool = &booleanType{
		parent: xsAnyType,
	}
	xsDecimal = &decimalType{
		parent: xsAnyType,
	}
	xsInteger = &integerType{
		parent: xsDecimal,
	}
	xsDateTime = &datetimeType{
		parent: xsAnyType,
	}
	xsDate = &dateType{
		parent: xsDateTime,
	}
)

type untypedType struct {
	parent XdmType
}

func (*untypedType) Name() xml.QName        { return xml.QualifiedName("untyped", schemaNS) }
func (*untypedType) InstanceOf(e Expr) bool { return true }
func (*untypedType) Cast(e Expr) Expr       { return e }
func (*untypedType) Castable(e Expr) bool   { return true }

type anyType struct {
	parent XdmType
}

func (*anyType) Name() xml.QName        { return xml.QualifiedName("any", schemaNS) }
func (*anyType) InstanceOf(e Expr) bool { return true }
func (*anyType) Cast(e Expr) Expr       { return e }
func (*anyType) Castable(e Expr) bool   { return true }

type anyAtomicType struct {
	parent XdmType
}

func (*anyAtomicType) Name() xml.QName        { return xml.QualifiedName("anyAtomic", schemaNS) }
func (*anyAtomicType) InstanceOf(e Expr) bool { return true }
func (*anyAtomicType) Cast(e Expr) Expr       { return e }
func (*anyAtomicType) Castable(e Expr) bool   { return true }

type stringType struct {
	parent XdmType
}

func (*stringType) Name() xml.QName        { return xml.QualifiedName("string", schemaNS) }
func (*stringType) InstanceOf(e Expr) bool { return false }
func (*stringType) Cast(e Expr) Expr       { return e }
func (*stringType) Castable(e Expr) bool   { return false }

type decimalType struct {
	parent XdmType
}

func (*decimalType) Name() xml.QName        { return xml.QualifiedName("decimal", schemaNS) }
func (*decimalType) InstanceOf(e Expr) bool { return false }
func (*decimalType) Cast(e Expr) Expr       { return e }
func (*decimalType) Castable(e Expr) bool   { return false }

type integerType struct {
	parent XdmType
}

func (*integerType) Name() xml.QName        { return xml.QualifiedName("integer", schemaNS) }
func (*integerType) InstanceOf(e Expr) bool { return false }
func (*integerType) Cast(e Expr) Expr       { return e }
func (*integerType) Castable(e Expr) bool   { return false }

type booleanType struct {
	parent XdmType
}

func (*booleanType) Name() xml.QName        { return xml.QualifiedName("boolean", schemaNS) }
func (*booleanType) InstanceOf(e Expr) bool { return false }
func (*booleanType) Cast(e Expr) Expr       { return e }
func (*booleanType) Castable(e Expr) bool   { return false }

type datetimeType struct {
	parent XdmType
}

func (*datetimeType) Name() xml.QName        { return xml.QualifiedName("dateTime", schemaNS) }
func (*datetimeType) InstanceOf(e Expr) bool { return false }
func (*datetimeType) Cast(e Expr) Expr       { return e }
func (*datetimeType) Castable(e Expr) bool   { return false }

type dateType struct {
	parent XdmType
}

func (*dateType) Name() xml.QName        { return xml.QualifiedName("date", schemaNS) }
func (*dateType) InstanceOf(e Expr) bool { return false }
func (*dateType) Cast(e Expr) Expr       { return e }
func (*dateType) Castable(e Expr) bool   { return false }

func toString(value any) (string, error) {
	switch v := value.(type) {
	case string:
		return v, nil
	case []byte:
		return string(v), nil
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64), nil
	case bool:
		return strconv.FormatBool(v), nil
	case time.Time:
		return v.Format("2006-01-02"), nil
	default:
		return "", ErrType
	}
}

func toInt(value any) (int64, error) {
	return castToInt(value)
}

func toFloat(value any) (float64, error) {
	return castToFloat(value)
}

func toBool(v any) bool {
	switch v := v.(type) {
	case bool:
		return v
	case float64:
		return v != 0
	case string:
		return len(v) > 0
	case time.Time:
		return !v.IsZero()
	default:
		return false
	}
}

func toTime(value any) (time.Time, error) {
	return castToTime(value)
}

var ErrCast = errors.New("value can not be cast to target type")

func castToTime(val any) (time.Time, error) {
	if t, ok := val.(time.Time); ok {
		return t, nil
	}
	if f, ok := val.(float64); ok {
		return time.UnixMilli(int64(f)), nil
	}
	str, ok := val.(string)
	if !ok {
		return time.Time{}, ErrCast
	}
	w, err := time.Parse("2006-01-02", str)
	if err != nil {
		err = ErrCast
	}
	return w, err
}

func castToInt(val any) (int64, error) {
	switch v := val.(type) {
	case int64:
		return v, nil
	case float64:
		return int64(v), nil
	case string:
		return strconv.ParseInt(v, 0, 64)
	case time.Time:
		return v.Unix(), nil
	default:
		return 0, nil
	}
}

func castToFloat(val any) (float64, error) {
	if f, ok := val.(float64); ok {
		return f, nil
	}
	if t, ok := val.(time.Time); ok {
		return float64(t.Unix()), nil
	}
	str, ok := val.(string)
	if !ok {
		return 0, ErrCast
	}
	f, err := strconv.ParseFloat(str, 64)
	if err != nil {
		err = ErrCast
	}
	return f, err
}

func castToBool(val any) (bool, error) {
	if b, ok := val.(bool); ok {
		return b, nil
	}
	str, ok := val.(string)
	if !ok {
		return false, ErrCast
	}
	b, err := strconv.ParseBool(str)
	if err != nil {
		err = ErrCast
	}
	return b, err
}
