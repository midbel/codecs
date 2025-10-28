package xpath

func FromRoot(expr Expr) Expr {
	var base current
	return fromBase(expr, base)
}

func atRoot(expr Expr) bool {
	e, ok := expr.(step)
	if !ok {
		return false
	}
	switch e := e.curr.(type) {
	case step:
		return atRoot(e)
	case current:
		return true
	case root:
		return true
	default:
		return false
	}
}

func updateNS(expr Expr, ns string) Expr {
	switch e := expr.(type) {
	case name:
		e.Space = ns
		return e
	case query:
		e.expr = updateNS(e.expr, ns)
		return e
	case union:
		for i := range e.all {
			e.all[i] = updateNS(e.all[i], ns)
		}
		return e
	case intersect:
		for i := range e.all {
			e.all[i] = updateNS(e.all[i], ns)
		}
		return e
	case except:
		for i := range e.all {
			e.all[i] = updateNS(e.all[i], ns)
		}
		return e
	case step:
		e.curr = updateNS(e.curr, ns)
		e.next = updateNS(e.next, ns)
		return e
	case filter:
		e.expr = updateNS(e.expr, ns)
		return e
	case axis:
		e.next = updateNS(e.next, ns)
		return e
	case call:
		for i := range e.args {
			e.args[i] = updateNS(e.args[i], ns)
		}
		return e
	default:
		return expr
	}
}

func fromBase(expr, base Expr) Expr {
	switch e := expr.(type) {
	case query:
		e.expr = fromBase(e.expr, base)
		return e
	case union:
		for i := range e.all {
			e.all[i] = fromBase(e.all[i], base)
		}
		return e
	case intersect:
		for i := range e.all {
			e.all[i] = fromBase(e.all[i], base)
		}
		return e
	case except:
		for i := range e.all {
			e.all[i] = fromBase(e.all[i], base)
		}
		return e
	case step:
		if atRoot(e) {
			return e
		}
		e.curr = fromBase(e.curr, base)
		return e
	case filter:
		if atRoot(e.expr) {
			return e
		}
		e.expr = fromBase(e.expr, base)
		return e
	case axis:
		return transform(e.next, base)
	case call:
		for i := range e.args {
			e.args[i] = fromBase(e.args[i], base)
		}
		return e
	default:
		return expr
	}
}

func transform(expr Expr, base Expr) Expr {
	a := axis{
		kind: descendantSelfAxis,
		next: typeNode{},
	}
	expr = step{
		curr: base,
		next: step{
			curr: a,
			next: expr,
		},
	}
	return expr
}
