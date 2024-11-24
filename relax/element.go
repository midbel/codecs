package relax

type Arity int8

const (
	ZeroOrMore Arity = 1 << iota
	ZeroOrOne
	OneOrMore
)

type QName struct {
	Space string
	Local string
}

type Attribute struct {
	QName
}

type Element struct {
	QName
	Arity
	Attrs    []*Attribute
	Elements []*Element
}
