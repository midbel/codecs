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
	var (
		str   = strconv.FormatInt(value, radix)
		chars = []byte(str)
		out   bytes.Buffer
		ptr   int
		grp   byte
	)
	slices.Reverse(chars)
	for i := len(picture) - 1; i >= 0; i-- {
		switch picture[i] {
		case '0', '#':
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
			out.WriteByte(picture[i])
		default:
			return "", fmt.Errorf("unexpected character in picture")
		}
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
	chars = out.Bytes()
	slices.Reverse(chars)
	return string(chars), nil
}

func formatNumber(value float64, picture string) (string, error) {
	return "", nil
}

func formatDate(value time.Time, picture string) (string, error) {
	return "", nil
}

func formatDateTime(value time.Time, picture string) (string, error) {
	return "", nil
}
