package relax

type Arity int8

const (
	ZeroOrMore Arity = 1 << iota
	ZeroOrOne
	OneOrMore
)

type Pattern interface{}

type Grammar struct {
	Start Pattern
	List  map[string]Pattern
}

type QName struct {
	Space string
	Local string
}

type Link struct {
	Ident string
	Arity
}

type Attribute struct {
	QName
	Arity
	Value Pattern
}

type Element struct {
	QName
	Arity
	Value    Pattern
	Patterns []Pattern
}

type Text struct{}

type Empty struct{}

type Enum struct {
	List []string
}
