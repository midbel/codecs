package relax

type Arity int8

const (
	ZeroOrMore Arity = 1 << iota
	ZeroOrOne
	OneOrMore
)

type Pattern interface{}

type QName struct {
	Space string
	Local string
}

type Grammar struct {
	Start Pattern
}

type Link struct {
	Ident string
	Arity
}

type Reference struct {
	Ident string
	Arity
	Pattern
}

type Attribute struct {
	QName
	Arity
	Value Pattern
}

type Element struct {
	QName
	Arity
	Value      Pattern
	Attributes []Pattern
	Elements   []Pattern
}

type Alternative struct{}

type Text struct{}

type Empty struct{}

type Enum struct {
	List []string
}
