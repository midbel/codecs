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
	Type string
	List []string
}

type Element struct {
	QName
	Arity
	Type     string
	List     []string
	Attrs    []*Attribute
	Elements []*Element
}
