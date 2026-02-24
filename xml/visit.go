package xml

type Visitor interface {
	VisitElement(*Element)
	VisitPI(*Instruction)
	VisitAttribute(*Attribute)
	VisitText(*Text)
	VisitCharData(*CharData)
	VisitComment(*Comment)
}

type VisitableNode interface {
	Accept(v Visitor)
}
