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

var executers map[xml.QName]ExecuteFunc

func init() {
	nest := func(exec ExecuteFunc) ExecuteFunc {
		fn := func(ctx *Context) (xpath.Sequence, error) {
			ctx.Enter(ctx)
			defer ctx.Leave(ctx)
			seq, err := exec(ctx.Nest())
			if err != nil {
				ctx.Error(ctx, err)
			}
			return seq, err
		}
		return fn
	}
	trace := func(exec ExecuteFunc) ExecuteFunc {
		fn := func(ctx *Context) (xpath.Sequence, error) {
			ctx.Enter(ctx)
			defer ctx.Leave(ctx)

			seq, err := exec(ctx)
			if err != nil {
				ctx.Error(ctx, err)
			}
			return seq, err
		}
		return fn
	}
	executers = map[xml.QName]ExecuteFunc{
		xml.QualifiedName("for-each", xsltNamespacePrefix):        nest(executeForeach),
		xml.QualifiedName("iterate", xsltNamespacePrefix):         nest(executeIterate),
		xml.QualifiedName("next-iteration", xsltNamespacePrefix):  nest(executeNextIteration),
		xml.QualifiedName("on-completion", xsltNamespacePrefix):   nest(executeOnCompletion),
		xml.QualifiedName("iterate", xsltNamespacePrefix):         nest(executeIterate),
		xml.QualifiedName("break", xsltNamespacePrefix):           trace(executeBreak),
		xml.QualifiedName("value-of", xsltNamespacePrefix):        trace(executeValueOf),
		xml.QualifiedName("call-template", xsltNamespacePrefix):   nest(executeCallTemplate),
		xml.QualifiedName("apply-templates", xsltNamespacePrefix): nest(executeApplyTemplates),
		xml.QualifiedName("apply-imports", xsltNamespacePrefix):   nest(executeApplyImport),
		xml.QualifiedName("if", xsltNamespacePrefix):              nest(executeIf),
		xml.QualifiedName("choose", xsltNamespacePrefix):          nest(executeChoose),
		xml.QualifiedName("when", xsltNamespacePrefix):            trace(executeWhen),
		xml.QualifiedName("otherwise", xsltNamespacePrefix):       trace(executeOtherwise),
		xml.QualifiedName("where-populated", xsltNamespacePrefix): trace(executeWherePopulated),
		xml.QualifiedName("on-empty", xsltNamespacePrefix):        trace(executeOnEmpty),
		xml.QualifiedName("on-not-empty", xsltNamespacePrefix):    trace(executeOnNotEmpty),
		xml.QualifiedName("try", xsltNamespacePrefix):             nest(executeTry),
		xml.QualifiedName("catch", xsltNamespacePrefix):           trace(executeCatch),
		xml.QualifiedName("variable", xsltNamespacePrefix):        trace(executeVariable),
		xml.QualifiedName("result-document", xsltNamespacePrefix): trace(executeResultDocument),
		xml.QualifiedName("source-document", xsltNamespacePrefix): nest(executeSourceDocument),
		xml.QualifiedName("with-param", xsltNamespacePrefix):      trace(executeWithParam),
		xml.QualifiedName("copy", xsltNamespacePrefix):            trace(executeCopy),
		xml.QualifiedName("copy-of", xsltNamespacePrefix):         trace(executeCopyOf),
		xml.QualifiedName("sequence", xsltNamespacePrefix):        trace(executeSequence),
		xml.QualifiedName("element", xsltNamespacePrefix):         trace(executeElement),
		xml.QualifiedName("attribute", xsltNamespacePrefix):       trace(executeAttribute),
		xml.QualifiedName("text", xsltNamespacePrefix):            trace(executeText),
		xml.QualifiedName("comment", xsltNamespacePrefix):         trace(executeComment),
		xml.QualifiedName("message", xsltNamespacePrefix):         trace(executeMessage),
		xml.QualifiedName("fallback", xsltNamespacePrefix):        trace(executeFallback),
		xml.QualifiedName("merge", xsltNamespacePrefix):           trace(executeMerge),
		xml.QualifiedName("for-each-group", xsltNamespacePrefix):  trace(executeForeachGroup),
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
	}
	if len(elem.Nodes) == 0 {
		return nil, nil
	}
	seq, err := executeConstructor(ctx, elem.Nodes, 0)
	if err != nil {
		return nil, err
	}
	ctx.DefineExprParam(ident, xpath.NewValueFromSequence(seq))
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
	var sub *Context
	if x, ok := tpl.(interface{ mergeContext(*Context) *Context }); ok {
		sub = x.mergeContext(ctx)
	} else {
		sub = ctx.Copy()
	}
	if err := applyParams(sub); err != nil {
		return nil, ctx.errorWithContext(err)
	}
	nodes, err := tpl.Execute(sub)
	if err != nil {
		return nil, err
	}
	var seq xpath.Sequence
	for i := range nodes {
		seq.Append(xpath.NewNodeItem(nodes[i]))
	}
	return seq, nil
}

func executeForeachGroup(ctx *Context) (xpath.Sequence, error) {
	elem, err := getElementFromNode(ctx.XslNode)
	if err != nil {
		return nil, ctx.errorWithContext(err)
	}
	query, err := getAttribute(elem, "select")
	if err != nil {
		return nil, err
	}

	items, err := ctx.ExecuteQuery(query, ctx.ContextNode)
	if err != nil {
		return nil, err
	}

	if len(items) == 0 {
		return executeConstructor(ctx, elem.Nodes, AllowOnEmpty|AllowOnNonEmpty)
	}

	key, err := getAttribute(elem, "group-by")
	if err != nil {
		return nil, err
	}
	grpby, err := ctx.CompileQuery(key)
	if err != nil {
		return nil, err
	}
	groups := make(map[string]xpath.Sequence)
	for i := range items {
		is, err := grpby.Find(items[i].Node())
		if err != nil {
			return nil, err
		}
		key := is[0].Value().(string)
		groups[key] = append(groups[key], items[i])
	}

	seq := xpath.NewSequence()
	for key, items := range groups {
		defineForeachGroupBuiltins(ctx, key, items)
		others, err := executeConstructor(ctx, elem.Nodes, AllowOnEmpty|AllowOnNonEmpty)
		if err != nil {
			return nil, err
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

func getSequenceFromSource(ctx *Context, node xml.Node) (string, xpath.Sequence, error) {
	elem, err := getElementFromNode(node)
	if err != nil {
		return "", nil, ctx.errorWithContext(err)
	}

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
		items, err1 := ctx.ExecuteQuery(query, ctx.ContextNode)
		if err1 != nil {
			return "", nil, err1
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
	case withSource:
		items, err1 := ctx.ExecuteQuery(query, ctx.ContextNode)
		if err1 != nil {
			return "", nil, err1
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
	if len(elem.Nodes) == 0 {
		err := fmt.Errorf("at least one merge-key should be given")
		return "", nil, ctx.errorWithContext(err)
	}
	for _, n := range elem.Nodes {
		if n.QualifiedName() != ctx.getQualifiedName("merge-key") {
			err := fmt.Errorf("%s: unexpected element", n.QualifiedName())
			return "", nil, ctx.errorWithContext(err)
		}
	}
	return ident, seq, err
}

func executeMerge(ctx *Context) (xpath.Sequence, error) {
	elem, err := getElementFromNode(ctx.XslNode)
	if err != nil {
		return nil, ctx.errorWithContext(err)
	}
	var (
		action xml.Node
		groups = make(map[string][]MergedItem)
	)

	for i, n := range elem.Nodes {
		if n.QualifiedName() != ctx.getQualifiedName("merge-source") {
			action = n
			break
		}
		el := n.(*xml.Element)
		ident, _ := getAttribute(el, "name")
		if err != nil {
			ident = fmt.Sprintf("source-%03d", i)
		}
		var items xpath.Sequence
		if query, err := getAttribute(el, "select"); err == nil {
			items, err = ctx.ExecuteQuery(query, ctx.ContextNode)
			if err != nil {
				return nil, err
			}
		} else {
			return nil, err
		}
		if len(el.Nodes) == 0 {
			return nil, fmt.Errorf("missing xsl:merge-key element")
		}
		if query, err := getAttribute(el.Nodes[0].(*xml.Element), "select"); err != nil {
			return nil, err
		} else {
			grp, err := ctx.CompileQuery(query)
			if err != nil {
				return nil, err
			}
			for i := range items {
				is, err := grp.Find(items[i].Node())
				if err != nil {
					return nil, err
				}
				mit := MergedItem{
					Item:   items[i],
					Source: ident,
					Key:    fmt.Sprint(is[0].Value()),
				}
				groups[mit.Key] = append(groups[mit.Key], mit)
			}
		}
	}
	if action.QualifiedName() != ctx.getQualifiedName("merge-action") {
		return nil, fmt.Errorf("merge-action expected")
	}
	elem, ok := action.(*xml.Element)
	if !ok {
		return nil, fmt.Errorf("merge-action: expected xml element")
	}

	var (
		keys = slices.Collect(maps.Keys(groups))
		seq  xpath.Sequence
	)
	slices.Sort(keys)
	for _, key := range keys {
		nested := ctx.Nest()
		defineMergeBuiltins(nested, key, groups[key])
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
	query, err := getAttribute(elem, "select")
	if err == nil {
		seq, err := ctx.ExecuteQuery(query, ctx.ContextNode)
		if err != nil {
			return nil, err
		}
		return seq, errBreak
	}
	seq, err := executeConstructor(ctx, elem.Nodes, 0)
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

func executeOnCompletion(ctx *Context) (xpath.Sequence, error) {
	return nil, nil
}

func executeIterate(ctx *Context) (xpath.Sequence, error) {
	elem, err := getElementFromNode(ctx.XslNode)
	if err != nil {
		return nil, ctx.errorWithContext(err)
	}
	items, err := executeSelect(ctx, elem)
	if err != nil {
		return nil, err
	}
	var seq xpath.Sequence
	for _, i := range items {
		var (
			sub   = ctx.Nest()
			nodes []xml.Node
			atEnd xml.Node
		)
		for i, n := range elem.Nodes {
			if n.QualifiedName() != ctx.getQualifiedName("param") {
				nodes = slices.Clone(elem.Nodes[i:])
				break
			}
			_, err := transformNode(sub.WithXsl(n))
			if err != nil {
				return nil, err
			}
		}
		if n := len(nodes); n > 0 {
			last := nodes[0]
			if last.QualifiedName() == ctx.getQualifiedName("on-completion") {
				atEnd = last
				nodes = nodes[1:]
			}
		}
		others, err := executeConstructor(sub.WithXpath(i.Node()), nodes, 0)
		if err != nil {
			if errors.Is(err, errIterate) {
				continue
			}
			if errors.Is(err, errBreak) {
				break
			}
			return nil, err
		}
		seq.Concat(others)
		if atEnd != nil {
			rest, err := transformNode(ctx.WithXsl(atEnd))
			if err != nil {
				return nil, err
			}
			seq.Concat(rest)
		}
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
	if t, ok := ctx.Tracer.(interface{ Println(string) }); ok {
		t.Println(strings.Join(parts, ""))
	}

	if quit, err := getAttribute(elem, "terminate"); err == nil && quit == "yes" {
		return nil, ErrTerminate
	}
	return nil, nil
}

func executeEvaluate(ctx *Context) (xpath.Sequence, error) {
	return nil, errImplemented
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
			if !isEmpty(seq) {
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
				return nil, err
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
		curr.Append(seq[i].Node())
	}
	return xpath.Singleton(curr), nil
}

func executeAttribute(ctx *Context) (xpath.Sequence, error) {
	return nil, errImplemented
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

func applySort(node xml.Node, items []xpath.Item) (iter.Seq[xpath.Item], error) {
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
		var sub *Context
		if x, ok := tpl.(interface{ mergeContext(*Context) *Context }); ok {
			sub = x.mergeContext(ctx.WithXpath(datum))
		} else {
			sub = ctx.Copy()
		}
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

func defineForeachGroupBuiltins(nested *Context, key string, seq xpath.Sequence) {
	currentGrp := func(_ xpath.Context, _ []xpath.Expr) (xpath.Sequence, error) {
		return seq, nil
	}
	currentKey := func(_ xpath.Context, _ []xpath.Expr) (xpath.Sequence, error) {
		i := xpath.NewLiteralItem(key)
		return xpath.Singleton(i), nil
	}

	nested.Builtins.Define("current-group", currentGrp)
	nested.Builtins.Define("fn:current-group", currentGrp)
	nested.Builtins.Define("current-grouping-key", currentKey)
	nested.Builtins.Define("fn:current-grouping-key", currentKey)
}

func defineMergeBuiltins(nested *Context, key string, items []MergedItem) {
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
	nested.Builtins.Define("current-merge-group", currentGrp)
	nested.Builtins.Define("fn:current-merge-group", currentGrp)
	nested.Builtins.Define("current-merge-key", currentKey)
	nested.Builtins.Define("fn:current-merge-key", currentKey)
}
