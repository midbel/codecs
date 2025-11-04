package xpath

func FromRoot(expr Expr) Expr {
	return fromRoot(expr)
}

func fromRoot(expr Expr) Expr {
	switch e := expr.(type) {
	case query:
		e.expr = fromRoot(e.expr)
		return e
	case root:
	case current:
	case union:
		for i := range e.all {
			e.all[i] = fromRoot(e.all[i])
		}
		return e
	case intersect:
		for i := range e.all {
			e.all[i] = fromRoot(e.all[i])
		}
		return e
	case except:
		for i := range e.all {
			e.all[i] = fromRoot(e.all[i])
		}
		return e
	case call:
		for i := range e.args {
			e.args[i] = fromRoot(e.args[i])
		}
		return e
	case step:
		if isRoot(e) {
			return e
		}
		return wrapExpr(e)
	default:
		expr = wrapExpr(e)
	}
	return expr
}

func isRoot(expr Expr) bool {
	switch e := expr.(type) {
	case step:
		return isRoot(e.curr)
	case root:
		return true
	default:
		return false
	}
}

func wrapExpr(expr Expr) Expr {
	switch expr.(type) {
	case step, axis, name, typeText, typeNode, typeComment, typeDocument, typeInstruction, typeAttribute, typeElement:
	default:
		return expr
	}
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
