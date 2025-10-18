package xpath

import (
	"bytes"
	"fmt"
	"slices"
	"strconv"
)

func formatInteger(value int64, picture string) (string, error) {
	var (
		str   = strconv.FormatInt(value, 10)
		chars = []byte(str)
		out   bytes.Buffer
	)
	slices.Reverse(chars)
	for i, j := len(picture)-1, 0; i >= 0; i-- {
		fmt.Println("i", i, j)
		switch picture[i] {
		case '0', '#':
			if j >= len(chars) {
				out.WriteByte('0')
			} else {
				out.WriteByte(chars[j])
			}
			j++
		case '.', ',':
			if out.Len()%3 != 0 {
				return "", fmt.Errorf("wrong position for thousand separator")
			}
			out.WriteByte(picture[i])
		default:
			return "", fmt.Errorf("unexpected character in picture")
		}
	}
	chars = out.Bytes()
	slices.Reverse(chars)
	return string(chars), nil
}

func formatNumber(value float64, picture string) (string, error) {
	return "", nil
}
