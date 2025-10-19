package xpath

import (
	"errors"
	"strconv"
	"time"
)

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
