package xpath

func fromRoot(expr Expr) Expr {
	transform := func(expr Expr) Expr {
		return step{
			curr: root{},
			next: step{
				curr: axis{
					kind: descendantSelfAxis,
					next: typeNode{},
				},
				next: expr,
			},
		}
	}
	switch e := expr.(type) {
	case root:
	case current:
	case step:
		_, ok := e.curr.(root)
		if !ok {
			break
		}
		s, ok := e.next.(step)
		if !ok {
			break
		}
		a, ok := s.curr.(axis)
		if !ok {
			break
		}
		if a.kind == descendantSelfAxis {
			break
		}
		expr = transform(e)
	default:
		expr = transform(e)
	}
	return expr
}
