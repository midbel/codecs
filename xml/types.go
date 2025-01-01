package xml

import (
	"errors"
	"time"
)

var ErrCast = errors.New("value can not be cast to target type")

func castToDate(str string) (time.Time, error) {
	w, err := time.Parse("2006-01-02", str)
	if err != nil {
		err = ErrCast
	}
	return w, err
}
