package xml

import (
	"errors"
	"strconv"
	"time"
)

var ErrCast = errors.New("value can not be cast to target type")

func castToDate(val any) (time.Time, error) {
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
