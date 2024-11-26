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
	Arity
	Type string
	List []string
}

type Element struct {
	QName
	Arity
	Type       string
	Attributes []*Attribute
	Elements   []*Element
}

type Text struct{}

type Empty struct{}

type Enum struct {
	List []string
}

type Type struct {
	Name string
	Constraint
}

type Constraint struct {
	Length    int
	MinLength int
	MaxLength int
	Pattern   string
	Enum      []string
}
