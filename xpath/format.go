package xpath

import (
	"bytes"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"
)

func formatInteger(value int64, picture string) (string, error) {
	radix := 10
	if rx, rest, ok := strings.Cut(picture, "^"); ok {
		picture = rest
		switch rx {
		case "8", "10", "16":
			radix, _ = strconv.Atoi(rx)
		default:
			return "", fmt.Errorf("unsupported radix")
		}
	}
	if value < 0 {
		value = -value
	}
	var (
		str   = strconv.FormatInt(value, radix)
		chars = []byte(str)
		out   bytes.Buffer
		ptr   int
		grp   byte
		prev  byte
	)
	slices.Reverse(chars)
	if picture[len(picture)-1] == '%' {
		picture = picture[:len(picture)-1]
		out.WriteByte('%')
	}
	for i := len(picture) - 1; i >= 0; i-- {
		switch picture[i] {
		case '0', '1', '2', '3', '4', '5', '6', '7', '8', '9', '#':
			if ptr >= len(chars) {
				if picture[i] != '#' {
					out.WriteByte('0')
				}
			} else {
				out.WriteByte(chars[ptr])
			}
			ptr++
		case '.', ',':
			if grp != 0 && picture[i] != grp {
				return "", fmt.Errorf("inconsistent use of thousand separator")
			}
			grp = picture[i]
			if ptr%3 != 0 {
				return "", fmt.Errorf("wrong position for thousand separator")
			}
			if prev == picture[i] {
				return "", fmt.Errorf("two consecutive thousand separator not allowed")
			}
			out.WriteByte(picture[i])
		default:
			return "", fmt.Errorf("unexpected character in picture")
		}
		prev = picture[i]
	}
	if ptr < len(chars) {
		for _, c := range chars[ptr:] {
			if grp != 0 && ptr%3 == 0 {
				out.WriteByte(grp)
			}
			out.WriteByte(c)
			ptr++
		}
	}
	if value < 0 {
		out.WriteByte('-')
	}
	chars = out.Bytes()
	slices.Reverse(chars)
	return string(chars), nil
}

func formatNumber(value float64, picture string) (string, error) {
	positive, negative, ok := strings.Cut(picture, ";")
	if ok && value < 0 {
		picture = negative
	} else {
		picture = positive
	}
	return "", nil
}

func formatDate(value time.Time, picture string) (string, error) {
	return "", nil
}

func formatDateTime(value time.Time, picture string) (string, error) {
	return "", nil
}
