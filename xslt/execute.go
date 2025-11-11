package xslt

import (
	"errors"
	"fmt"
	"iter"
	"maps"
	"slices"
	"sort"
	"strings"

	"github.com/midbel/codecs/xml"
	"github.com/midbel/codecs/xpath"
)

type ExecuteFunc func(*Context) (xpath.Sequence, error)

var (
	executers         map[xml.QName]ExecuteFunc
	iterateExecuters  map[xml.QName]ExecuteFunc
	mergeExecuters    map[xml.QName]ExecuteFunc
	forgroupExecuters map[xml.QName]ExecuteFunc
)

func init() {
	nest := func(exec ExecuteFunc) ExecuteFunc {
		fn := func(ctx *Context) (xpath.Sequence, error) {
			ns := ctx.ResetXpathNamespace()
			defer ctx.SetXpathNamespace(ns)
			return exec(ctx.Nest())
		}
		return fn
	}
	trace := func(exec ExecuteFunc) ExecuteFunc {
		fn := func(ctx *Context) (xpath.Sequence, error) {
			ns := ctx.ResetXpathNamespace()
			defer ctx.SetXpathNamespace(ns)
			return exec(ctx)
		}
		return fn
	}
	iterateExecuters = map[xml.QName]ExecuteFunc{
		xsltQualifiedName("next-iteration"): trace(executeNextIteration),
		xsltQualifiedName("break"):          trace(executeBreak),
	}
	executers = map[xml.QName]ExecuteFunc{
		xsltQualifiedName("for-each"):               nest(executeForeach),
		xsltQualifiedName("iterate"):                nest(executeIterate),
		xsltQualifiedName("value-of"):               trace(executeValueOf),
		xsltQualifiedName("call-template"):          nest(executeCallTemplate),
		xsltQualifiedName("apply-templates"):        nest(executeApplyTemplates),
		xsltQualifiedName("apply-imports"):          nest(executeApplyImport),
		xsltQualifiedName("if"):                     nest(executeIf),
		xsltQualifiedName("choose"):                 nest(executeChoose),
		xsltQualifiedName("when"):                   trace(executeWhen),
		xsltQualifiedName("otherwise"):              trace(executeOtherwise),
		xsltQualifiedName("where-populated"):        trace(executeWherePopulated),
		xsltQualifiedName("on-empty"):               trace(executeOnEmpty),
		xsltQualifiedName("on-not-empty"):           trace(executeOnNotEmpty),
		xsltQualifiedName("try"):                    nest(executeTry),
		xsltQualifiedName("catch"):                  trace(executeCatch),
		xsltQualifiedName("variable"):               trace(executeVariable),
		xsltQualifiedName("result-document"):        trace(executeResultDocument),
		xsltQualifiedName("source-document"):        nest(executeSourceDocument),
		xsltQualifiedName("with-param"):             trace(executeWithParam),
		xsltQualifiedName("copy"):                   trace(executeCopy),
		xsltQualifiedName("copy-of"):                trace(executeCopyOf),
		xsltQualifiedName("sequence"):               trace(executeSequence),
		xsltQualifiedName("document"):               trace(executeDocument),
		xsltQualifiedName("processing-instruction"): trace(executePI),
		xsltQualifiedName("element"):                trace(executeElement),
		xsltQualifiedName("attribute"):              trace(executeAttribute),
		xsltQualifiedName("text"):                   trace(executeText),
		xsltQualifiedName("comment"):                trace(executeComment),
		xsltQualifiedName("namespace"):              trace(executeNamespace),
		xsltQualifiedName("message"):                trace(executeMessage),
		xsltQualifiedName("fallback"):               trace(executeFallback),
		xsltQualifiedName("merge"):                  trace(executeMerge),
		xsltQualifiedName("for-each-group"):         trace(executeForeachGroup),
		xsltQualifiedName("assert"):                 trace(executeAssert),
		xsltQualifiedName("evaluate"):               trace(executeEvaluate),
	}
}

func registerExecuters(set map[xml.QName]ExecuteFunc) bool {
	for qn, fn := range set {
		_, ok := executers[qn]
		if ok {
			return false
		}
		executers[qn] = fn
	}
	return true
}

func unregisterExecuters(set map[xml.QName]ExecuteFunc) {
	for qn := range set {
		delete(executers, qn)
	}
}

func executeSourceDocument(ctx *Context) (xpath.Sequence, error) {
	elem, err := getElementFromNode(ctx.XslNode)
	if err != nil {
		return nil, ctx.errorWithContext(err)
	}
	file, err := getAttribute(elem, "href")
	if err != nil {
		return nil, ctx.errorWithContext(err)
	}
	doc, err := ctx.LoadDocument(file)
	if err != nil {
		return nil, ctx.errorWithContext(err)
	}
	return executeConstructor(ctx.WithXpath(doc), elem.Nodes, AllowOnEmpty|AllowOnNonEmpty)
}

func executeResultDocument(ctx *Context) (xpath.Sequence, error) {
	elem, err := getElementFromNode(ctx.XslNode)
	if err != nil {
		return nil, ctx.errorWithContext(err)
	}
	seq, err := executeConstructor(ctx, elem.Nodes, AllowOnEmpty|AllowOnNonEmpty)
	if err != nil {
		return nil, err
	}
	var doc xml.Document
	for i := range seq {
		doc.Nodes = append(doc.Nodes, seq[i].Node())
	}

	file, err := getAttribute(elem, "href")
	if err != nil {
		return nil, ctx.errorWithContext(err)
	}
	format, _ := getAttribute(elem, "format")
	if err := writeDocument(file, format, &doc, ctx.Stylesheet); err != nil {
		return nil, ctx.errorWithContext(err)
	}
	return nil, nil
}

func executeVariable(ctx *Context) (xpath.Sequence, error) {
	elem, err := getElementFromNode(ctx.XslNode)
	if err != nil {
		return nil, ctx.errorWithContext(err)
	}
	ident, err := getAttribute(elem, "name")
	if err != nil {
		return nil, ctx.errorWithContext(err)
	}
	var seq xpath.Sequence
	if query, err1 := getAttribute(elem, "select"); err1 == nil {
		if len(elem.Nodes) > 0 {
			return nil, fmt.Errorf("select attribute can not be used with children")
		}
		seq, err = ctx.ExecuteQuery(query, ctx.ContextNode)
	} else {
		for _, n := range slices.Clone(elem.Nodes) {
			c := cloneNode(n)
			if c == nil {
				continue
			}
			res, err := transformNode(ctx.WithXsl(c))
			if err != nil {
				return nil, ctx.errorWithContext(err)
			}
			seq.Concat(res)
		}
	}
	if err != nil {
		return nil, ctx.errorWithContext(err)
	}
	ctx.Define(ident, xpath.NewValueFromSequence(seq))
	return nil, nil
}

func executeWithParam(ctx *Context) (xpath.Sequence, error) {
	elem, err := getElementFromNode(ctx.XslNode)
	if err != nil {
		return nil, ctx.errorWithContext(err)
	}
	ident, err := getAttribute(elem, "name")
	if err != nil {
		return nil, ctx.errorWithContext(err)
	}
	if query, err := getAttribute(elem, "select"); err == nil {
		if len(elem.Nodes) != 0 {
			return nil, fmt.Errorf("select attribute can not be used with children")
		}
		ctx.EvalParam(ident, query, ctx.ContextNode)
	} else {
		if len(elem.Nodes) == 0 {
			err := fmt.Errorf("no value given to param %q", ident)
			return nil, ctx.errorWithContext(err)
		}
		seq, err := executeConstructor(ctx, elem.Nodes, 0)
		if err != nil {
			return nil, err
		}
		ctx.DefineExprParam(ident, xpath.NewValueFromSequence(seq))
	}
	return nil, nil
}

func executeApplyImport(ctx *Context) (xpath.Sequence, error) {
	return executeApply(ctx, ctx.MatchImport)
}

func executeApplyTemplates(ctx *Context) (xpath.Sequence, error) {
	return executeApply(ctx, ctx.Match)
}

func executeCallTemplate(ctx *Context) (xpath.Sequence, error) {
	elem, err := getElementFromNode(ctx.XslNode)
	if err != nil {
		return nil, ctx.errorWithContext(err)
	}
	name, err := getAttribute(elem, "name")
	if err != nil {
		return nil, ctx.errorWithContext(err)
	}
	mode, err := getAttribute(elem, "mode")
	if err == nil {
		ctx = ctx.WithMode(mode)
	}
	tpl, err := ctx.Find(name, mode)
	if err != nil {
		return nil, err
	}
	sub := ctx.Nest()
	if t, ok := tpl.(interface{ FillWithDefaults(*Context) *Context }); ok {
		sub = t.FillWithDefaults(sub)
	}
	if t, ok := tpl.(*Template); ok {
		sub.Env = sub.Env.Merge(t.env)
	}
	if err := applyParams(sub); err != nil {
		return nil, ctx.errorWithContext(err)
	}
	call, ok := tpl.(interface {
		Call(*Context) ([]xml.Node, error)
	})
	if !ok {
		err := fmt.Errorf("template %q can not be called", name)
		return nil, ctx.errorWithContext(err)
	}
	nodes, err := call.Call(sub)
	if err != nil {
		return nil, err
	}
	var seq xpath.Sequence
	for i := range nodes {
		seq.Append(xpath.NewNodeItem(nodes[i]))
	}
	return seq, nil
}

type GroupItem struct {
	Hash  string
	Value xpath.Sequence
	Items xpath.Sequence
}

type SortGroupItem struct {
	GroupItem
	Sort xpath.Sequence
	Dir  string
}

func executeForeachGroup(ctx *Context) (xpath.Sequence, error) {
	elem, err := getElementFromNode(ctx.XslNode)
	if err != nil {
		return nil, ctx.errorWithContext(err)
	}
	query, err := getAttribute(elem, "select")
	if err != nil {
		return nil, ctx.errorWithContext(err)
	}

	items, err := ctx.ExecuteQuery(query, ctx.ContextNode)
	if err != nil {
		return nil, ctx.errorWithContext(err)
	}

	if len(items) == 0 {
		return executeConstructor(ctx, elem.Nodes, AllowOnEmpty|AllowOnNonEmpty)
	}

	key, err := getAttribute(elem, "group-by")
	if err != nil {
		return nil, ctx.errorWithContext(err)
	}
	grpby, err := ctx.CompileQuery(key)
	if err != nil {
		return nil, ctx.errorWithContext(err)
	}
	var (
		groups = make(map[string]GroupItem)
		keys   []string
	)
	for i := range items {
		is, err := grpby.Find(items[i].Node())
		if err != nil {
			return nil, ctx.errorWithContext(err)
		}
		sh := is.CanonicalizeString()
		gi, ok := groups[sh]
		if !ok {
			gi.Hash = sh
			gi.Value = is
			keys = append(keys, sh)
		}
		gi.Items = append(gi.Items, items[i])
		groups[sh] = gi
	}

	var (
		seq   xpath.Sequence
		order string
		nodes = slices.Clone(elem.Nodes)
	)

	if len(nodes) > 0 && nodes[0].QualifiedName() == ctx.getQualifiedName("sort") {
		elem, err := getElementFromNode(nodes[0])
		if err != nil {
			return nil, ctx.errorWithContext(err)
		}
		query, err = getAttribute(elem, "select")
		if err != nil {
			return nil, ctx.errorWithContext(err)
		}
		order, _ = getAttribute(elem, "order")
		nodes = nodes[1:]
	}
	var list []SortGroupItem
	for _, key := range keys {
		var (
			sit SortGroupItem
			gi  = groups[key]
			sub = ctx.Copy()
		)
		sit.GroupItem = gi
		defineForeachGroupBuiltins(sub, gi.Value, gi.Items)
		if query != "" {
			seq, err := sub.ExecuteQuery(query, sub.ContextNode)
			if err != nil {
				return nil, ctx.errorWithContext(err)
			}
			if seq.Len() == 0 {
				sit.Sort = xpath.Singleton(0)
			} else {
				sit.Sort = seq
			}
		}
		list = append(list, sit)
	}
	slices.SortFunc(list, func(g1, g2 SortGroupItem) int {
		res := g1.Sort.Compare(&g2.Sort)
		if order == "descending" {
			res = -res
		}
		return res
	})
	for _, gi := range list {
		defineForeachGroupBuiltins(ctx, gi.Value, gi.Items)
		others, err := executeConstructor(ctx, nodes, AllowOnEmpty|AllowOnNonEmpty)
		if err != nil {
			return nil, ctx.errorWithContext(err)
		}
		seq.Concat(others)
	}
	return seq, nil
}

type MergedItem struct {
	xpath.Item
	Key    string
	Source string
}

func getMergeItems(ctx *Context, elem *xml.Element) (string, xpath.Sequence, error) {
	ident, _ := getAttribute(elem, "name")
	if ident == "" {
		ident = ctx.makeIdent()
	}

	query, err := getAttribute(elem, "select")
	if err != nil {
		return "", nil, ctx.errorWithContext(err)
	}

	expr, err := ctx.CompileQuery(query)
	if err != nil {
		return "", nil, ctx.errorWithContext(err)
	}

	var (
		withItem   = hasAttribute("for-each-item", elem.Attrs)
		withSource = hasAttribute("for-each-source", elem.Attrs)
		seq        xpath.Sequence
	)
	switch {
	case withItem && withSource:
		err := fmt.Errorf("for-each-item and for-each-source can not be used simultaneously")
		return "", nil, ctx.errorWithContext(err)
	case withItem:
		source, err := getAttribute(elem, "for-each-item")
		if err != nil {
			return "", nil, err
		}
		items, err := ctx.ExecuteQuery(source, ctx.ContextNode)
		if err != nil {
			return "", nil, err
		}
		for i := range items {
			others, err1 := expr.Find(items[i].Node())
			if err != nil {
				return "", nil, err1
			}
			seq.Concat(others)
		}
	case withSource:
		source, err := getAttribute(elem, "for-each-source")
		if err != nil {
			return "", nil, err
		}
		items, err := ctx.ExecuteQuery(source, ctx.ContextNode)
		if err != nil {
			return "", nil, err
		}
		for i := range items {
			doc, err1 := xml.ParseFile(toString(items[i]))
			if err1 != nil {
				return "", nil, ctx.errorWithContext(err1)
			}
			others, err1 := expr.Find(doc)
			if err1 != nil {
				return "", nil, err
			}
			seq.Concat(others)
		}
	default:
		seq, err = expr.Find(ctx.ContextNode)
	}
	return ident, seq, err
}

func getSequenceFromSource(ctx *Context, node xml.Node) (map[string][]MergedItem, error) {
	elem, err := getElementFromNode(node)
	if err != nil {
		return nil, ctx.errorWithContext(err)
	}

	ident, seq, err := getMergeItems(ctx, elem)
	if err != nil {
		return nil, err
	}

	if len(elem.Nodes) == 0 {
		err := fmt.Errorf("at least one merge-key should be given")
		return nil, ctx.errorWithContext(err)
	}
	var list []xpath.Expr
	for _, n := range elem.Nodes {
		if n.QualifiedName() != ctx.getQualifiedName("merge-key") {
			err := fmt.Errorf("%s: unexpected element", n.QualifiedName())
			return nil, ctx.errorWithContext(err)
		}
		elem, err := getElementFromNode(n)
		if err != nil {
			return nil, ctx.errorWithContext(err)
		}
		query, err := getAttribute(elem, "select")
		if err != nil {
			return nil, ctx.errorWithContext(err)
		}
		expr, err := ctx.CompileQuery(query)
		if err != nil {
			return nil, err
		}
		list = append(list, expr)
	}
	groups := make(map[string][]MergedItem)
	for i := range seq {
		var keys []string
		for _, e := range list {
			res, err := e.Find(seq[i].Node())
			if err != nil {
				return nil, err
			}
			if len(res) != 1 {
				err := fmt.Errorf("merge-key should only produce one result")
				return nil, ctx.errorWithContext(err)
			}
			keys = append(keys, toString(res[0]))
		}
		if len(keys) != len(list) {
			continue
		}
		m := MergedItem{
			Source: ident,
			Key:    strings.Join(keys, "/"),
			Item:   seq[i],
		}
		groups[m.Key] = append(groups[m.Key], m)
	}
	return groups, nil
}

func executeMerge(ctx *Context) (xpath.Sequence, error) {
	elem, err := getElementFromNode(ctx.XslNode)
	if err != nil {
		return nil, ctx.errorWithContext(err)
	}
	var (
		nodes  = slices.Clone(elem.Nodes)
		action xml.Node
		groups = make(map[string][]MergedItem)
	)
	ix := slices.IndexFunc(nodes, func(n xml.Node) bool {
		return n.QualifiedName() == ctx.getQualifiedName("merge-action")
	})
	if ix < 0 {
		err := fmt.Errorf("missing merge-action element")
		return nil, ctx.errorWithContext(err)
	}
	if ix != len(nodes)-1 {
		err := fmt.Errorf("merge-action should be the last element")
		return nil, ctx.errorWithContext(err)
	}
	action = nodes[ix]
	nodes = nodes[:ix]

	for _, n := range nodes {
		if n.QualifiedName() != ctx.getQualifiedName("merge-source") {
			err := fmt.Errorf("%s: unexpected element", n.QualifiedName())
			return nil, ctx.errorWithContext(err)
		}
		others, err := getSequenceFromSource(ctx, n)
		if err != nil {
			return nil, err
		}
		maps.Copy(groups, others)
	}
	elem, err = getElementFromNode(action)
	if err != nil {
		return nil, ctx.errorWithContext(err)
	}
	var (
		keys = slices.Collect(maps.Keys(groups))
		seq  xpath.Sequence
	)
	slices.Sort(keys)
	for _, key := range keys {
		nested := ctx.Nest()
		defineMergeBuiltins(nested, key, keys, groups[key])
		res, err := executeConstructor(nested, elem.Nodes, AllowOnEmpty|AllowOnNonEmpty)
		if err != nil {
			return nil, err
		}
		seq.Concat(res)
	}
	return seq, nil
}

func executeBreak(ctx *Context) (xpath.Sequence, error) {
	elem, err := getElementFromNode(ctx.XslNode)
	if err != nil {
		return nil, err
	}
	var seq xpath.Sequence
	if query, err1 := getAttribute(elem, "select"); err1 == nil {
		if len(elem.Nodes) > 0 {
			return nil, fmt.Errorf("using select and children nodes is not allowed")
		}
		seq, err = ctx.ExecuteQuery(query, ctx.ContextNode)
	} else {
		seq, err = executeConstructor(ctx, elem.Nodes, 0)
	}
	if err != nil {
		return nil, err
	}
	return seq, errBreak
}

func executeNextIteration(ctx *Context) (xpath.Sequence, error) {
	if err := applyParams(ctx); err != nil {
		return nil, err
	}
	return nil, errIterate
}

func executeIterate(ctx *Context) (xpath.Sequence, error) {
	if registerExecuters(iterateExecuters) {
		defer unregisterExecuters(iterateExecuters)
	}

	elem, err := getElementFromNode(ctx.XslNode)
	if err != nil {
		return nil, ctx.errorWithContext(err)
	}
	if len(elem.Nodes) == 0 {
		return nil, fmt.Errorf("%s: empty", elem.QualifiedName())
	}
	query, err := getAttribute(elem, "select")
	if err != nil {
		return nil, err
	}
	items, err := ctx.ExecuteQuery(query, ctx.ContextNode)
	if err != nil {
		return nil, err
	}
	var (
		onComplete xml.Node
		nodes      = slices.Clone(elem.Nodes)
	)
	if nodes[0].QualifiedName() == ctx.getQualifiedName("on-completion") {
		onComplete = nodes[0]
		nodes = nodes[1:]
	}
	nest := ctx.Nest()
	for i := range nodes {
		if nodes[i].QualifiedName() != ctx.getQualifiedName("param") {
			nodes = nodes[i:]
			break
		}
		elem, err := getElementFromNode(nodes[i])
		if err != nil {
			return nil, err
		}
		ident, err := getAttribute(elem, "name")
		if err != nil {
			return nil, err
		}
		if query, err := getAttribute(elem, "select"); err == nil {
			if len(elem.Nodes) > 0 {
				return nil, fmt.Errorf("using select and children nodes is not allowed")
			}
			if err := nest.DefineParam(ident, query); err != nil {
				return nil, err
			}
		} else {
			seq, err := executeConstructor(nest, elem.Nodes, 0)
			if err != nil {
				return nil, err
			}
			nest.DefineExprParam(ident, xpath.NewValueFromSequence(seq))
		}
	}

	var seq xpath.Sequence
	for _, i := range items {
		sub := nest.WithXpath(i.Node())
		for _, n := range nodes {
			c := cloneNode(n)
			if c == nil {
				continue
			}
			others, err := transformNode(sub.WithXsl(c))
			if err != nil {
				if errors.Is(err, errIterate) {
					continue
				} else if errors.Is(err, errBreak) {
					return others, nil
				}
				return nil, err
			}
			seq.Concat(others)
		}
	}
	if onComplete != nil {
		elem, err := getElementFromNode(onComplete)
		if err != nil {
			return nil, err
		}
		return executeConstructor(nest, elem.Nodes, 0)
	}
	return seq, nil
}

func executeForeach(ctx *Context) (xpath.Sequence, error) {
	elem, err := getElementFromNode(ctx.XslNode)
	if err != nil {
		return nil, ctx.errorWithContext(err)
	}
	query, err := getAttribute(elem, "select")
	if err != nil {
		return nil, ctx.errorWithContext(err)
	}

	var (
		nodes = slices.Clone(elem.Nodes)
		it    iter.Seq[xpath.Item]
	)

	items, err := ctx.ExecuteQuery(query, ctx.ContextNode)
	if err != nil {
		return nil, ctx.errorWithContext(err)
	}

	if len(items) == 0 {
		if n := len(nodes); n > 0 && nodes[n-1].QualifiedName() == ctx.getQualifiedName("on-empty") {
			return transformNode(ctx.WithXsl(nodes[n-1]))
		}
		return nil, nil
	}
	if len(nodes) > 0 && nodes[0].QualifiedName() == ctx.getQualifiedName("sort") {
		it, err = applySort(nodes[0], items)
		if err != nil {
			return nil, ctx.errorWithContext(err)
		}
		nodes = nodes[1:]
	} else {
		it = slices.Values(items)
	}

	seq := xpath.NewSequence()
	for i := range it {
		node := i.Node()
		others, err := executeConstructor(ctx.WithXpath(node), nodes, AllowOnEmpty|AllowOnNonEmpty)
		if err != nil {
			return nil, err
		}
		seq.Concat(others)
	}
	return seq, nil
}

func executeCatch(ctx *Context) (xpath.Sequence, error) {
	elem, err := getElementFromNode(ctx.XslNode)
	if err != nil {
		return nil, err
	}
	return executeConstructor(ctx, elem.Nodes, 0)
}

func executeTry(ctx *Context) (xpath.Sequence, error) {
	elem, err := getElementFromNode(ctx.XslNode)
	if err != nil {
		return nil, err
	}
	var (
		catch []xml.Node
		body  []xml.Node
		seq   xpath.Sequence
	)
	ix := slices.IndexFunc(elem.Nodes, func(n xml.Node) bool {
		return n.QualifiedName() == ctx.getQualifiedName("catch")
	})
	if ix >= 0 {
		body = slices.Clone(elem.Nodes[:ix])
		catch = slices.Clone(elem.Nodes[ix:])
	} else {
		body = slices.Clone(elem.Nodes)
	}
	if query, err1 := getAttribute(elem, "select"); err1 == nil {
		if len(body) > 0 {
			err := fmt.Errorf("select attribute can not be used with children")
			return nil, ctx.errorWithContext(err)
		}
		seq, err = ctx.ExecuteQuery(query, ctx.ContextNode)
	} else {
		if !errors.Is(err, errMissed) {
			return nil, ctx.errorWithContext(err)
		}
		seq, err = executeConstructor(ctx, body, 0)
	}
	if err == nil {
		return seq, nil
	}
	if !Catchable(err) {
		return nil, err
	}
	for i := range catch {
		seq, err := transformNode(ctx.WithXsl(catch[i]))
		if err != nil {
			if errors.Is(err, errBreak) {
				return seq, nil
			}
			return nil, err
		}
	}
	return nil, nil
}

func executeAssert(ctx *Context) (xpath.Sequence, error) {
	return nil, errImplemented
}

func executeIf(ctx *Context) (xpath.Sequence, error) {
	elem, err := getElementFromNode(ctx.XslNode)
	if err != nil {
		return nil, err
	}
	test, err := getAttribute(elem, "test")
	if err != nil {
		return nil, err
	}
	ok, err := ctx.TestNode(test, ctx.ContextNode)
	if err != nil {
		return nil, err
	}
	var seq xpath.Sequence
	if ok {
		seq, err = executeConstructor(ctx, elem.Nodes, 0)
	}
	return seq, err
}

func executeWhen(ctx *Context) (xpath.Sequence, error) {
	elem, err := getElementFromNode(ctx.XslNode)
	if err != nil {
		return nil, ctx.errorWithContext(err)
	}
	test, err := getAttribute(elem, "test")
	if err != nil {
		return nil, err
	}
	ok, err := ctx.TestNode(test, ctx.ContextNode)
	if err != nil {
		return nil, err
	}

	var seq xpath.Sequence
	if ok {
		seq, err = executeConstructor(ctx, elem.Nodes, 0)
		if err == nil {
			err = errBreak
		}
	}
	return seq, err
}

func executeOtherwise(ctx *Context) (xpath.Sequence, error) {
	elem, err := getElementFromNode(ctx.XslNode)
	if err != nil {
		return nil, ctx.errorWithContext(err)
	}
	return executeConstructor(ctx, elem.Nodes, 0)
}

func executeChoose(ctx *Context) (xpath.Sequence, error) {
	elem, err := getElementFromNode(ctx.XslNode)
	if err != nil {
		return nil, ctx.errorWithContext(err)
	}
	var (
		other xml.Node
		nodes = slices.Clone(elem.Nodes)
	)
	ix := slices.IndexFunc(nodes, func(n xml.Node) bool {
		return n.QualifiedName() == ctx.getQualifiedName("otherwise")
	})
	if ix >= 0 && ix == len(nodes)-1 {
		other = nodes[ix]
		nodes = nodes[:ix]
	}
	for i := range nodes {
		if nodes[i].QualifiedName() != ctx.getQualifiedName("when") {
			err := fmt.Errorf("%s: unexpected element - want xsl:when", nodes[i].QualifiedName())
			return nil, ctx.errorWithContext(err)
		}
		seq, err := transformNode(ctx.WithXsl(nodes[i]))
		if err != nil {
			if errors.Is(err, errBreak) {
				return seq, nil
			}
			return nil, err
		}
	}
	if other != nil {
		return transformNode(ctx.WithXsl(other))
	}
	return nil, nil
}

func executeValueOf(ctx *Context) (xpath.Sequence, error) {
	elem, err := getElementFromNode(ctx.XslNode)
	if err != nil {
		return nil, ctx.errorWithContext(err)
	}
	sep, err := getAttribute(elem, "separator")
	if err != nil {
		sep = " "
	}
	var items xpath.Sequence
	if query, err1 := getAttribute(elem, "select"); err1 != nil {
		if !errors.Is(err1, errMissed) {
			return nil, ctx.errorWithContext(err1)
		}
		items, err = executeConstructor(ctx, elem.Nodes, 0)
	} else {
		if len(elem.Nodes) > 0 {
			err := fmt.Errorf("select attribute can not be used with children")
			return nil, ctx.errorWithContext(err)
		}
		items, err = ctx.ExecuteQuery(query, ctx.ContextNode)
	}
	if err != nil {
		return nil, ctx.errorWithContext(err)
	}
	if len(items) == 0 {
		return xpath.Singleton(xml.NewText("")), nil
	}

	var str strings.Builder
	for i := range items {
		if i > 0 {
			str.WriteString(sep)
		}
		str.WriteString(toString(items[i]))
	}
	return xpath.Singleton(xml.NewText(str.String())), nil
}

func executeCopy(ctx *Context) (xpath.Sequence, error) {
	return executeCopyOf(ctx)
}

func executeCopyOf(ctx *Context) (xpath.Sequence, error) {
	elem, err := getElementFromNode(ctx.XslNode)
	if err != nil {
		return nil, ctx.errorWithContext(err)
	}
	query, err := getAttribute(elem, "select")
	if err != nil {
		return nil, ctx.errorWithContext(err)
	}
	items, err := ctx.ExecuteQuery(query, ctx.ContextNode)
	if err != nil {
		return nil, ctx.errorWithContext(err)
	}
	seq := xpath.NewSequence()
	for i := range items {
		c := cloneNode(items[i].Node())
		if c != nil {
			seq.Append(xpath.NewNodeItem(c))
		}
	}
	return seq, nil
}

func executeMessage(ctx *Context) (xpath.Sequence, error) {
	elem, err := getElementFromNode(ctx.XslNode)
	if err != nil {
		return nil, ctx.errorWithContext(err)
	}
	var parts []string
	for _, n := range elem.Nodes {
		parts = append(parts, n.Value())
	}
	if quit, err := getAttribute(elem, "terminate"); err == nil && quit == "yes" {
		return nil, ErrTerminate
	}
	return nil, nil
}

func executeEvaluate(ctx *Context) (xpath.Sequence, error) {
	elem, err := getElementFromNode(ctx.XslNode)
	if err != nil {
		return nil, ctx.errorWithContext(err)
	}
	query, err := getAttribute(elem, "xpath")
	if err != nil {
		return nil, err
	}
	sub := ctx.Nest()
	if q, err := getAttribute(elem, "context-item"); err == nil {
		seq, err := sub.ExecuteQuery(q, ctx.ContextNode)
		if err != nil {
			return nil, err
		}
		if !seq.Singleton() && !seq.Empty() {
			err := fmt.Errorf("expected singleton sequence for context item")
			return nil, ctx.errorWithContext(err)
		}
		if seq.Empty() {
			sub = sub.WithXpath(nil)
		} else {
			sub = sub.WithXpath(seq[0].Node())
		}
	}
	var (
		nodes    = elem.Nodes
		fallback xml.Node
	)
	_ = fallback // check if evaluate is disabled
	if nodes[len(nodes)-1].QualifiedName() == ctx.getQualifiedName("fallback") {
		fallback = nodes[len(nodes)-1]
		nodes = nodes[:len(nodes)-1]
	}
	for _, n := range nodes {
		if n.QualifiedName() != ctx.getQualifiedName("with-param") {
			err := fmt.Errorf("%s: unexpected element - want xsl:with-param", n.QualifiedName())
			return nil, ctx.errorWithContext(err)
		}
		_, err := transformNode(sub.WithXsl(n))
		if err != nil {
			return nil, err
		}
	}
	return sub.ExecuteQuery(query, sub.ContextNode)
}

func getMatchingElements(ctx *Context, elem *xml.Element) (xml.Node, xml.Node, error) {
	var (
		match   xml.Node
		nomatch xml.Node
	)
	if len(elem.Nodes) == 0 {
		err := fmt.Errorf("at least one children expected")
		return nil, nil, ctx.errorWithContext(err)
	}
	if elem.Nodes[0].QualifiedName() == ctx.getQualifiedName("matching-substring") {
		match = elem.Nodes[0]
	} else if elem.Nodes[0].QualifiedName() == ctx.getQualifiedName("non-matching-substring") {
		nomatch = elem.Nodes[0]
	} else {
		err := fmt.Errorf("unexpected element")
		return nil, nil, ctx.errorWithContext(err)
	}
	if len(elem.Nodes) > 1 && match != nil {
		if elem.Nodes[1].QualifiedName() == ctx.getQualifiedName("non-matching-substring") {
			nomatch = elem.Nodes[1]
		}
	}
	return match, nomatch, nil
}

func executeAnalyzeString(ctx *Context) (xpath.Sequence, error) {
	// elem, err := getElementFromNode(ctx.XslNode)
	// if err != nil {
	// 	return nil, ctx.errorWithContext(err)
	// }
	// query, err := getAttribute(elem, "select")
	// if err != nil {
	// 	return nil, err
	// }
	// regex, err := getAttribute(elem, "regex")
	// if err != nil {
	// 	return nil, err
	// }
	// match, nomatch, err := getMatchingElements(ctx, elem)
	// if err != nil {
	// 	return nil, err
	// }
	return nil, errImplemented
}

func executeMatchingSubstring(ctx *Context) (xpath.Sequence, error) {
	return nil, errImplemented
}

func executeNonMatchingSubstring(ctx *Context) (xpath.Sequence, error) {
	return nil, errImplemented
}

func executeWherePopulated(ctx *Context) (xpath.Sequence, error) {
	elem, err := getElementFromNode(ctx.XslNode)
	if err != nil {
		return nil, ctx.errorWithContext(err)
	}
	return executeConstructor(ctx, elem.Nodes, AllowOnEmpty|AllowOnNonEmpty)
}

func executeOnEmpty(ctx *Context) (xpath.Sequence, error) {
	elem, err := getElementFromNode(ctx.XslNode)
	if err != nil {
		return nil, ctx.errorWithContext(err)
	}
	seq, err := executeSelect(ctx, elem)
	if !errors.Is(err, errMissed) {
		return seq, err
	}
	return executeConstructor(ctx, elem.Nodes, 0)
}

func executeOnNotEmpty(ctx *Context) (xpath.Sequence, error) {
	elem, err := getElementFromNode(ctx.XslNode)
	if err != nil {
		return nil, ctx.errorWithContext(err)
	}
	seq, err := executeSelect(ctx, elem)
	if !errors.Is(err, errMissed) {
		return seq, err
	}
	return executeConstructor(ctx, elem.Nodes, 0)
}

func executeSequence(ctx *Context) (xpath.Sequence, error) {
	elem, err := getElementFromNode(ctx.XslNode)
	if err != nil {
		return nil, ctx.errorWithContext(err)
	}
	seq, err := executeSelect(ctx, elem)
	if !errors.Is(err, errMissed) {
		return seq, err
	}
	return executeConstructor(ctx, elem.Nodes, AllowOnEmpty|AllowOnNonEmpty)
}

type constructorFlags int8

func (c constructorFlags) AllowEmpty() bool {
	return c&AllowOnEmpty != 0
}

func (c constructorFlags) AllowNotEmpty() bool {
	return c&AllowOnNonEmpty != 0
}

const (
	AllowOnEmpty constructorFlags = 1 << iota
	AllowOnNonEmpty
)

func executeConstructor(ctx *Context, nodes []xml.Node, options constructorFlags) (xpath.Sequence, error) {
	var (
		seq     xpath.Sequence
		pending []xml.Node
	)
	for i, n := range nodes {
		c := cloneNode(n)
		if c == nil {
			continue
		}
		switch {
		case c.QualifiedName() == ctx.getQualifiedName("on-empty"):
			if !options.AllowEmpty() {
				err := fmt.Errorf("%s is not allowed", c.QualifiedName())
				return nil, ctx.errorWithContext(err)
			}
			if i < len(nodes)-1 {
				err := fmt.Errorf("%s can only be the last child of node", c.QualifiedName())
				return nil, ctx.errorWithContext(err)
			}
			if !seq.Empty() {
				break
			}
			return transformNode(ctx.WithXsl(c))
		case c.QualifiedName() == ctx.getQualifiedName("on-non-empty"):
			if !options.AllowNotEmpty() {
				err := fmt.Errorf("%s is not allowed", c.QualifiedName())
				return nil, ctx.errorWithContext(err)
			}
			pending = append(pending, c)
		default:
			others, err := transformNode(ctx.WithXsl(c))
			if err != nil {
				return others, err
			}
			if !others.Empty() && len(pending) > 0 && options.AllowNotEmpty() {
				tmp, err := executeNodes(ctx, pending)
				if err != nil {
					return nil, err
				}
				seq.Concat(tmp)
				pending = nil
			}
			seq.Concat(others)
		}
	}
	if !seq.Empty() && options.AllowNotEmpty() {
		others, err := executeNodes(ctx, pending)
		if err != nil {
			return nil, err
		}
		seq.Concat(others)
	}
	return seq, nil
}

func executeNodes(ctx *Context, nodes []xml.Node) (xpath.Sequence, error) {
	var seq xpath.Sequence
	for i := range nodes {
		tmp, err := transformNode(ctx.WithXsl(nodes[i]))
		if err != nil {
			return nil, err
		}
		seq.Concat(tmp)
	}
	return seq, nil
}

func executeSelect(ctx *Context, elem *xml.Element) (xpath.Sequence, error) {
	query, err := getAttribute(elem, "select")
	if err == nil {
		if len(elem.Nodes) != 0 {
			return nil, fmt.Errorf("select attribute can not be used with children")
		}
		return ctx.ExecuteQuery(query, ctx.ContextNode)
	}
	return nil, err
}

func executePI(ctx *Context) (xpath.Sequence, error) {
	el, err := getElementFromNode(ctx.XslNode)
	if err != nil {
		return nil, err
	}
	ident, err := getAttribute(el, "name")
	if err != nil {
		return nil, err
	}
	qn, err := xml.ParseName(ident)
	if err != nil {
		return nil, ctx.errorWithContext(err)
	}
	if qn.LocalName() == "xml" {
		err := fmt.Errorf("processing-instruction can not have 'xml' name")
		return nil, ctx.errorWithContext(err)
	}
	var seq xpath.Sequence
	if query, err := getAttribute(el, "select"); err == nil {
		if len(el.Nodes) != 0 {
			return nil, fmt.Errorf("select attribute can not be used with children")
		}
		seq, err = ctx.ExecuteQuery(query, ctx.ContextNode)
	} else {
		seq, err = executeConstructor(ctx, el.Nodes, 0)
	}
	if err != nil || seq.Empty() {
		return nil, err
	}
	pi := xml.NewInstruction(qn)
	for _, i := range seq {
		a, ok := i.Node().(*xml.Attribute)
		if !ok {
			err := fmt.Errorf("expected attribute")
			return nil, ctx.errorWithContext(err)
		}
		pi.SetAttribute(*a)
	}
	return xpath.Singleton(pi), nil
}

func executeNamespace(ctx *Context) (xpath.Sequence, error) {
	el, err := getElementFromNode(ctx.XslNode)
	if err != nil {
		return nil, err
	}
	ident, err := getAttribute(el, "name")
	if err != nil {
		return nil, err
	}
	var seq xpath.Sequence
	if query, err := getAttribute(el, "select"); err == nil {
		if len(el.Nodes) != 0 {
			return nil, fmt.Errorf("select attribute can not be used with children")
		}
		seq, err = ctx.ExecuteQuery(query, ctx.ContextNode)
	} else {
		seq, err = executeConstructor(ctx, el.Nodes, 0)
	}
	if err != nil || seq.Empty() {
		return nil, err
	}
	str, err := seq.Atomize()
	if err != nil {
		return nil, err
	}
	a := xml.NewAttribute(xml.QualifiedName(ident, "xmlns"), strings.Join(str, " "))
	return xpath.Singleton(&a), nil
}

func executeDocument(ctx *Context) (xpath.Sequence, error) {
	el, err := getElementFromNode(ctx.XslNode)
	if err != nil {
		return nil, err
	}
	items, err := executeConstructor(ctx, el.Nodes, 0)
	if err != nil {
		return nil, err
	}
	doc := xml.EmptyDocument()
	for _, i := range items {
		doc.Nodes = append(doc.Nodes, i.Node())
	}
	return xpath.Singleton(doc), nil
}

func executeElement(ctx *Context) (xpath.Sequence, error) {
	elem, err := getElementFromNode(ctx.XslNode)
	if err != nil {
		return nil, ctx.errorWithContext(err)
	}
	ident, err := getAttribute(elem, "name")
	if err != nil {
		return nil, ctx.errorWithContext(err)
	}
	qn, err := xml.ParseName(ident)
	if err != nil {
		return nil, ctx.errorWithContext(err)
	}
	seq, err := executeConstructor(ctx, elem.Nodes, AllowOnEmpty|AllowOnNonEmpty)
	if err != nil {
		return nil, err
	}
	curr := xml.NewElement(qn)
	for i := range seq {
		n := seq[i].Node()
		curr.Append(n)
	}
	return xpath.Singleton(curr), nil
}

func executeAttribute(ctx *Context) (xpath.Sequence, error) {
	elem, err := getElementFromNode(ctx.XslNode)
	if err != nil {
		return nil, ctx.errorWithContext(err)
	}
	ident, err := getAttribute(elem, "name")
	if err != nil {
		return nil, ctx.errorWithContext(err)
	}
	qn, err := xml.ParseName(ident)
	if err != nil {
		return nil, ctx.errorWithContext(err)
	}
	var items xpath.Sequence
	if query, err := getAttribute(elem, "select"); err == nil {
		if len(elem.Nodes) != 0 {
			return nil, fmt.Errorf("select attribute can not be used with children")
		}
		items, err = ctx.ExecuteQuery(query, ctx.ContextNode)
	} else {
		items, err = executeConstructor(ctx, elem.Nodes, 0)
	}
	if err != nil {
		return nil, err
	}
	var value string
	if !items.Empty() {
		value = toString(items[0])
	}
	attr := xml.NewAttribute(qn, value)
	return xpath.Singleton(&attr), nil
}

func executeText(ctx *Context) (xpath.Sequence, error) {
	elem := xml.NewText(ctx.XslNode.Value())
	return xpath.Singleton(xpath.NewNodeItem(elem)), nil
}

func executeComment(ctx *Context) (xpath.Sequence, error) {
	elem := xml.NewComment(ctx.XslNode.Value())
	return xpath.Singleton(xpath.NewNodeItem(elem)), nil
}

func executeFallback(ctx *Context) (xpath.Sequence, error) {
	return nil, errImplemented
}

func applyParams(ctx *Context) error {
	elem, err := getElementFromNode(ctx.XslNode)
	if err != nil {
		return ctx.errorWithContext(err)
	}
	for _, n := range slices.Clone(elem.Nodes) {
		if n.QualifiedName() != ctx.getQualifiedName("with-param") {
			return fmt.Errorf("%s: invalid child node %s", ctx.XslNode.QualifiedName(), n.QualifiedName())
		}
		_, err := transformNode(ctx.WithXsl(n))
		if err != nil {
			return err
		}
	}
	return nil
}

func applySort(node xml.Node, items xpath.Sequence) (iter.Seq[xpath.Item], error) {
	elem, err := getElementFromNode(node)
	if err != nil {
		return nil, err
	}
	query, err := getAttribute(elem, "select")
	if err != nil {
		return nil, err
	}
	order, _ := getAttribute(elem, "order")
	return iterItems(items, query, order)
}

type matchFunc func(xml.Node, string) (Executer, error)

func executeApply(ctx *Context, match matchFunc) (xpath.Sequence, error) {
	nodes, err := getNodesForTemplate(ctx)
	if err != nil {
		return nil, err
	}
	if len(nodes) == 0 {
		return nil, nil
	}
	elem, err := getElementFromNode(ctx.XslNode)
	if err != nil {
		return nil, err
	}
	mode, err := getAttribute(elem, "mode")
	if err == nil {
		ctx = ctx.WithMode(mode)
	}
	var seq xpath.Sequence
	for _, datum := range nodes {
		tpl, err := match(datum, mode)
		if err != nil {
			return seq, err
		}
		sub := ctx.WithXpath(datum)
		if err := applyParams(sub); err != nil {
			return nil, err
		}
		res, err := tpl.Execute(sub)
		if err != nil {
			return nil, err
		}
		for i := range res {
			seq.Append(xpath.NewNodeItem(res[i]))
		}
	}
	return seq, nil
}

func iterItems(items []xpath.Item, orderBy, orderDir string) (iter.Seq[xpath.Item], error) {
	expr, err := xpath.CompileString(orderBy)
	if err != nil {
		return nil, err
	}
	getString := func(is []xpath.Item) string {
		if len(is) == 0 {
			return ""
		}
		val := is[0].Value()
		return val.(string)
	}
	var compare func(string, string) bool
	if orderDir == "descending" {
		compare = func(str1, str2 string) bool {
			return strings.Compare(str1, str2) >= 0
		}
	} else {
		compare = func(str1, str2 string) bool {
			return strings.Compare(str1, str2) < 0
		}
	}
	fn := func(yield func(xpath.Item) bool) {
		is := slices.Clone(items)
		sort.Slice(is, func(i, j int) bool {
			x1, err1 := expr.Find(is[i].Node())
			x2, err2 := expr.Find(is[j].Node())
			if err1 != nil || err2 != nil {
				return false
			}
			return compare(getString(x1), getString(x2))
		})
		for i := range is {
			if !yield(is[i]) {
				break
			}
		}
	}
	return fn, nil
}

func getNodesForTemplate(ctx *Context) ([]xml.Node, error) {
	elem, err := getElementFromNode(ctx.XslNode)
	if err != nil {
		return nil, ctx.errorWithContext(err)
	}
	var res []xml.Node
	if query, err := getAttribute(elem, "select"); err == nil {
		items, err := ctx.ExecuteQuery(query, ctx.ContextNode)
		if err != nil {
			return nil, err
		}
		for i := range items {
			res = append(res, items[i].Node())
		}
	} else {
		res = []xml.Node{cloneNode(ctx.ContextNode)}
	}
	return res, nil
}

func defineForeachGroupBuiltins(ctx *Context, key, items xpath.Sequence) {
	currentGrp := func(_ xpath.Context, _ []xpath.Expr) (xpath.Sequence, error) {
		return items, nil
	}
	currentKey := func(_ xpath.Context, _ []xpath.Expr) (xpath.Sequence, error) {
		return key, nil
	}

	ctx.RegisterFunc("current-group", currentGrp)
	ctx.RegisterFunc("current-grouping-key", currentKey)
}

func defineMergeBuiltins(ctx *Context, key string, all []string, items []MergedItem) {
	currentKey := func(_ xpath.Context, _ []xpath.Expr) (xpath.Sequence, error) {
		return xpath.Singleton(key), nil
	}
	currentGrp := func(ctx xpath.Context, args []xpath.Expr) (xpath.Sequence, error) {
		if len(args) > 1 {
			return nil, fmt.Errorf("too many arguments")
		}
		var (
			seq xpath.Sequence
			grp string
		)
		if len(args) == 1 {
			names, err := args[0].Find(ctx)
			if err != nil {
				return nil, err
			}
			if names.Empty() {
				return nil, fmt.Errorf("no group available")
			}
			grp = fmt.Sprint(names[0].Value())
		}
		for i := range items {
			if grp != "" && items[i].Source != grp {
				continue
			}
			seq.Append(items[i].Item)
		}
		return seq, nil
	}
	mergeKeys := func(_ xpath.Context, _ []xpath.Expr) (xpath.Sequence, error) {
		var seq xpath.Sequence
		for i := range all {
			seq.Append(xpath.NewLiteralItem(all[i]))
		}
		return seq, nil
	}
	ctx.RegisterFunc("current-merge-group", currentGrp)
	ctx.RegisterFunc("current-merge-key", currentKey)
	ctx.RegisterFunc("angle:merge-keys", mergeKeys)
}
