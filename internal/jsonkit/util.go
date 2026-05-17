package jsonkit

func IsComment(c, k rune) bool {
	return c == '/' && c == k
}

func IsHex(c rune) bool {
	return IsNumber(c) || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')
}

func IsNumber(c rune) bool {
	return c >= '0' && c <= '9'
}

func IsLower(c rune) bool {
	return c >= 'a' && c <= 'z'
}

func IsUpper(c rune) bool {
	return c >= 'A' && c <= 'Z'
}

func IsLetter(c rune) bool {
	return IsLower(c) || IsUpper(c)
}

func IsAlpha(c rune) bool {
	return IsLetter(c) || IsNumber(c) || c == '_'
}

func IsApos(c rune) bool {
	return c == '\''
}

func IsQuote(c rune) bool {
	return c == '"'
}

func IsBackQuote(c rune) bool {
	return c == '`'
}

func IsDelim(c rune) bool {
	return c == '{' || c == '}' || c == '[' || c == ']' || c == ',' || c == ':'
}

func IsNL(c rune) bool {
	return c == '\n' || c == '\r'
}

func IsOperator(c rune) bool {
	return c == '!' || c == '=' || c == '<' || c == '>' ||
		c == '&' || c == '*' || c == '/' || c == '%' || c == '-' ||
		c == '+' || c == '.' || c == '?' || c == ':'
}

func IsTransform(c rune) bool {
	return c == '|'
}

func IsDollar(c rune) bool {
	return c == '$'
}
