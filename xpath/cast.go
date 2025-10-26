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
	Cast(any) (Sequence, error)
	Castable(any) bool

	setParent(XdmType)
	append(XdmType)
}

var (
	xsUntyped  = &untypedType{}
	xsAny      = &anyType{}
	xsAtomic   = &anyAtomicType{}
	xsString   = &stringType{}
	xsBool     = &booleanType{}
	xsDecimal  = &decimalType{}
	xsInteger  = &integerType{}
	xsDateTime = &datetimeType{}
	xsDate     = &dateType{}
)

var supportedTypes = map[xml.QName]XdmType{
	xsUntyped.Name():  xsUntyped,
	xsAny.Name():      xsAny,
	xsAtomic.Name():   xsAtomic,
	xsString.Name():   xsString,
	xsBool.Name():     xsBool,
	xsDecimal.Name():  xsDecimal,
	xsInteger.Name():  xsInteger,
	xsDateTime.Name(): xsDateTime,
	xsDate.Name():     xsDate,
}

func init() {
	for k, t := range supportedTypes {
		delete(supportedTypes, k)
		k.Uri = schemaNS
		supportedTypes[k] = t
	}

	xsUntyped.append(xsAny)
	xsAny.append(xsAtomic)
	xsAtomic.append(xsBool)
	xsAtomic.append(xsString)
	xsAtomic.append(xsDecimal)
	xsAtomic.append(xsDateTime)
	xsDecimal.append(xsInteger)
	xsDateTime.append(xsDate)
}

type untypedType struct {
	sub []XdmType
}

func (*untypedType) Name() xml.QName {
	return xml.QualifiedName("untyped", "xs")
}

func (*untypedType) InstanceOf(e Expr) bool {
	return true
}

func (*untypedType) Cast(any) (Sequence, error) {
	return nil, ErrImplemented
}

func (*untypedType) Castable(any) bool {
	return true
}

func (t *untypedType) subTypes() []XdmType {
	return t.sub
}

func (t *untypedType) setParent(parent XdmType) {
	// pass
}

func (t *untypedType) append(xt XdmType) {
	xt.setParent(t)
	t.sub = append(t.sub, xt)
}

type anyType struct {
	parent XdmType
	sub    []XdmType
}

func (*anyType) Name() xml.QName {
	return xml.QualifiedName("any", "xs")
}

func (t *anyType) InstanceOf(e Expr) bool {
	return instanceOf(e, t)
}

func (*anyType) Cast(v any) (Sequence, error) {
	return Singleton(v), nil
}

func (*anyType) Castable(any) bool {
	return true
}

func (t *anyType) derived() XdmType {
	return t.parent
}

func (t *anyType) subTypes() []XdmType {
	return t.sub
}

func (t *anyType) setParent(parent XdmType) {
	t.parent = parent
}

func (t *anyType) append(xt XdmType) {
	xt.setParent(t)
	t.sub = append(t.sub, xt)
}

type anyAtomicType struct {
	parent XdmType
	sub    []XdmType
}

func (*anyAtomicType) Name() xml.QName {
	return xml.QualifiedName("anyAtomic", "xs")
}

func (t *anyAtomicType) InstanceOf(e Expr) bool {
	return instanceOf(e, t)
}

func (*anyAtomicType) Cast(v any) (Sequence, error) {
	switch v.(type) {
	case int64, float64, bool, string, time.Time:
		return Singleton(v), nil
	default:
		return nil, ErrCast
	}
}

func (t *anyAtomicType) Castable(v any) bool {
	_, err := t.Cast(v)
	return err == nil
}

func (t *anyAtomicType) derived() XdmType {
	return t.parent
}

func (t *anyAtomicType) subTypes() []XdmType {
	return t.sub
}

func (t *anyAtomicType) setParent(parent XdmType) {
	t.parent = parent
}

func (t *anyAtomicType) append(xt XdmType) {
	xt.setParent(t)
	t.sub = append(t.sub, xt)
}

type stringType struct {
	parent XdmType
}

func (*stringType) Name() xml.QName {
	return xml.QualifiedName("string", "xs")
}

func (t *stringType) InstanceOf(e Expr) bool {
	return instanceOf(e, t)
}

func (*stringType) Cast(v any) (Sequence, error) {
	var str string
	switch v := v.(type) {
	case int64:
		str = strconv.FormatInt(v, 64)
	case float64:
		str = strconv.FormatFloat(v, 'f', -1, 64)
	case bool:
		str = strconv.FormatBool(v)
	case string:
		str = v
	case time.Time:
		str = v.Format(time.RFC3339)
	default:
		return nil, ErrCast
	}
	return Singleton(str), nil
}

func (t *stringType) Castable(v any) bool {
	_, err := t.Cast(v)
	return err == nil
}

func (t *stringType) derived() XdmType {
	return t.parent
}

func (t *stringType) setParent(parent XdmType) {
	t.parent = parent
}

func (t *stringType) append(xt XdmType) {
	// pass
}

type decimalType struct {
	parent XdmType
	sub    []XdmType
}

func (*decimalType) Name() xml.QName {
	return xml.QualifiedName("decimal", "xs")
}

func (t *decimalType) InstanceOf(e Expr) bool {
	return instanceOf(e, t)
}

func (*decimalType) Cast(v any) (Sequence, error) {
	var dec float64
	switch v := v.(type) {
	case int64:
		dec = float64(v)
	case float64:
		dec = v
	case bool:
		if v {
			dec += 1
		}
	case string:
		d, err := strconv.ParseFloat(v, 64)
		if err != nil {
			return nil, err
		}
		dec = d
	case time.Time:
		dec = float64(v.Unix())
	default:
		return nil, ErrCast
	}
	return Singleton(dec), nil
}

func (t *decimalType) Castable(v any) bool {
	_, err := t.Cast(v)
	return err == nil
}

func (t *decimalType) subTypes() []XdmType {
	return t.sub
}

func (t *decimalType) derived() XdmType {
	return t.parent
}

func (t *decimalType) setParent(parent XdmType) {
	t.parent = parent
}

func (t *decimalType) append(xt XdmType) {
	xt.setParent(t)
	t.sub = append(t.sub, xt)
}

type integerType struct {
	parent XdmType
}

func (*integerType) Name() xml.QName {
	return xml.QualifiedName("integer", "xs")
}

func (t *integerType) InstanceOf(e Expr) bool {
	return instanceOf(e, t)
}

func (*integerType) Cast(v any) (Sequence, error) {
	var dec int64
	switch v := v.(type) {
	case int64:
		dec = v
	case float64:
		dec = int64(v)
	case bool:
		if v {
			dec += 1
		}
	case string:
		d, err := strconv.ParseInt(v, 0, 64)
		if err != nil {
			return nil, err
		}
		dec = d
	case time.Time:
		dec = v.Unix()
	default:
		return nil, ErrCast
	}
	return Singleton(dec), nil
}

func (t *integerType) Castable(v any) bool {
	_, err := t.Cast(v)
	return err == nil
}

func (t *integerType) derived() XdmType {
	return t.parent
}

func (t *integerType) setParent(parent XdmType) {
	t.parent = parent
}

func (t *integerType) append(xt XdmType) {
	// pass
}

type booleanType struct {
	parent XdmType
}

func (*booleanType) Name() xml.QName {
	return xml.QualifiedName("boolean", "xs")
}

func (t *booleanType) InstanceOf(e Expr) bool {
	return instanceOf(e, t)
}

func (*booleanType) Cast(v any) (Sequence, error) {
	var res bool
	switch v := v.(type) {
	case int64:
		res = v != 0
	case float64:
		res = v != 0
	case bool:
		res = v
	case string:
		res = v != ""
	case time.Time:
		res = !v.IsZero()
	default:
		return nil, ErrCast
	}
	return Singleton(res), nil
}

func (t *booleanType) Castable(v any) bool {
	_, err := t.Cast(v)
	return err == nil
}

func (t *booleanType) derived() XdmType {
	return t.parent
}

func (t *booleanType) setParent(xt XdmType) {
	t.parent = xt
}

func (t *booleanType) append(xt XdmType) {
	// pass
}

type datetimeType struct {
	parent XdmType
	sub    []XdmType
}

func (*datetimeType) Name() xml.QName {
	return xml.QualifiedName("dateTime", "xs")
}

func (t *datetimeType) InstanceOf(e Expr) bool {
	return instanceOf(e, t)
}

func (*datetimeType) Cast(any) (Sequence, error) {
	return nil, ErrImplemented
}

func (*datetimeType) Castable(any) bool {
	return false
}

func (t *datetimeType) derived() XdmType {
	return t.parent
}

func (t *datetimeType) subTypes() []XdmType {
	return t.sub
}

func (t *datetimeType) setParent(parent XdmType) {
	t.parent = parent
}

func (t *datetimeType) append(xt XdmType) {
	xt.setParent(t)
	t.sub = append(t.sub, xt)
}

type dateType struct {
	parent XdmType
}

func (*dateType) Name() xml.QName {
	return xml.QualifiedName("date", "xs")
}

func (t *dateType) derived() XdmType {
	return t.parent
}

func (t *dateType) InstanceOf(e Expr) bool {
	return instanceOf(e, t)
}

func (*dateType) Cast(any) (Sequence, error) {
	return nil, ErrImplemented
}

func (*dateType) Castable(any) bool {
	return false
}

func (t *dateType) setParent(parent XdmType) {
	t.parent = parent
}

func (t *dateType) append(xt XdmType) {
	// pass
}

func instanceOf(expr Expr, typ XdmType) bool {
	t, ok := expr.(TypedExpr)
	if !ok {
		return false
	}
	return isInstanceOf(t.Type(), typ)
}

func isInstanceOf(source, target XdmType) bool {
	if source == target {
		return true
	}
	s, ok := target.(interface{ subTypes() []XdmType })
	if !ok {
		return false
	}
	for _, x := range s.subTypes() {
		if ok = isInstanceOf(source, x); ok {
			return ok
		}
	}
	return false
}

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
