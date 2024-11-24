package json

import "fmt"

type Position struct {
	Line   int
	Column int
}

type Token struct {
	Literal string
	Type    rune
	Position
}

func (t Token) String() string {
	var prefix string
	switch t.Type {
	case Transform:
		return "<transform>"
	case Doc:
		return "<document>"
	case Ternary:
		return "<ternary>"
	case Colon:
		return "<colon>"
	case BegGrp:
		return "<beg-grp>"
	case EndGrp:
		return "<end-grp>"
	case And:
		return "<and>"
	case Or:
		return "<or>"
	case In:
		return "<in>"
	case Add:
		return "<add>"
	case Sub:
		return "<subtract>"
	case Mul:
		return "<multiply>"
	case Div:
		return "<divide>"
	case Mod:
		return "<modulo>"
	case Eq:
		return "<equal>"
	case Ne:
		return "<not-equal>"
	case Lt:
		return "<lesser-than>"
	case Le:
		return "<lesser-eq>"
	case Gt:
		return "<greater-than>"
	case Ge:
		return "<greater-eq>"
	case Concat:
		return "<concat>"
	case Map:
		return "<map>"
	case Parent:
		return "<parent>"
	case Wildcard:
		return "<wildcard>"
	case Descent:
		return "<descend>"
	case Range:
		return "<range>"
	case EOF:
		return "<eof>"
	case BegArr:
		return "<beg-arr>"
	case EndArr:
		return "<end-arr>"
	case BegObj:
		return "<beg-obj>"
	case EndObj:
		return "<end-obj>"
	case Comma:
		return "<comma>"
	case Boolean:
		prefix = "boolean"
	case Null:
		return "<null>"
	case String:
		prefix = "string"
	case Number:
		prefix = "number"
	case Ident:
		prefix = "identifier"
	case Func:
		prefix = "function"
	case Comment:
		prefix = "comment"
	case Invalid:
		prefix = "invalid"
	}
	return fmt.Sprintf("%s(%s)", prefix, t.Literal)
}

const (
	EOF = -(1 + iota)
	BegArr
	EndArr
	BegObj
	EndObj
	Comma
	Colon
	Boolean
	Null
	String
	Number
	Ident
	Func
	Comment
	// query token
	Doc
	BegGrp
	EndGrp
	In
	And
	Or
	Add
	Sub
	Mul
	Div
	Mod
	Eq
	Ne
	Lt
	Le
	Gt
	Ge
	Concat
	Ternary
	Map
	Parent
	Wildcard
	Descent
	Range
	Transform
	// common
	Invalid
)
