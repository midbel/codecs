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
	case union:
		for i := range e.all {
			e.all[i] = fromRoot(e.all[i])
		}
		expr = e
	case intersect:
		for i := range e.all {
			e.all[i] = fromRoot(e.all[i])
		}
		expr = e
	case except:
		for i := range e.all {
			e.all[i] = fromRoot(e.all[i])
		}
		expr = e
	case step:
		if !isRoot(e.curr) {
			return transform(expr)
		}
		s, ok := e.next.(step)
		if !ok {
			return expr
		}
		a, ok := s.curr.(axis)
		if !ok {
			return transform(expr)
		}
		if a.kind != descendantSelfAxis {
			return transform(expr)
		}
	default:
		expr = transform(e)
	}
	return expr
}

func isRoot(expr Expr) bool {
	_, ok := expr.(root)
	return ok
}
