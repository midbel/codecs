package xslt

import (
	"fmt"
	"iter"
	"maps"
	"slices"
	"sort"
	"strings"

	"github.com/midbel/codecs/xml"
)

type ExecuteFunc func(*Context) (xml.Sequence, error)

var executers map[xml.QName]ExecuteFunc

func init() {
	nest := func(exec ExecuteFunc) ExecuteFunc {
		fn := func(ctx *Context) (xml.Sequence, error) {
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
		fn := func(ctx *Context) (xml.Sequence, error) {
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
		xml.QualifiedName("value-of", xsltNamespacePrefix):        trace(executeValueOf),
		xml.QualifiedName("call-template", xsltNamespacePrefix):   nest(executeCallTemplate),
		xml.QualifiedName("apply-templates", xsltNamespacePrefix): nest(executeApplyTemplates),
		xml.QualifiedName("apply-imports", xsltNamespacePrefix):   nest(executeApplyImport),
		xml.QualifiedName("if", xsltNamespacePrefix):              nest(executeIf),
		xml.QualifiedName("choose", xsltNamespacePrefix):          nest(executeChoose),
		xml.QualifiedName("where-populated", xsltNamespacePrefix): trace(executeWherePopulated),
		xml.QualifiedName("on-empty", xsltNamespacePrefix):        trace(executeOnEmpty),
		xml.QualifiedName("on-not-empty", xsltNamespacePrefix):    trace(executeOnNotEmpty),
		xml.QualifiedName("try", xsltNamespacePrefix):             nest(executeTry),
		xml.QualifiedName("variable", xsltNamespacePrefix):        trace(executeVariable),
		xml.QualifiedName("result-document", xsltNamespacePrefix): trace(executeResultDocument),
		xml.QualifiedName("source-document", xsltNamespacePrefix): nest(executeSourceDocument),
		xml.QualifiedName("import", xsltNamespacePrefix):          trace(executeImport),
		xml.QualifiedName("include", xsltNamespacePrefix):         trace(executeInclude),
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

func executeImport(ctx *Context) (xml.Sequence, error) {
	elem, err := getElementFromNode(ctx.XslNode)
	if err != nil {
		return nil, ctx.errorWithContext(err)
	}
	file, err := getAttribute(elem, "href")
	if err != nil {
		return nil, ctx.errorWithContext(err)
	}
	return nil, ctx.ImportSheet(file)
}

func executeInclude(ctx *Context) (xml.Sequence, error) {
	elem, err := getElementFromNode(ctx.XslNode)
	if err != nil {
		return nil, ctx.errorWithContext(err)
	}
	file, err := getAttribute(elem, "href")
	if err != nil {
		return nil, ctx.errorWithContext(err)
	}
	return nil, ctx.IncludeSheet(file)
}

func executeSourceDocument(ctx *Context) (xml.Sequence, error) {
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

	seq := xml.NewSequence()
	for _, n := range slices.Clone(elem.Nodes) {
		c := cloneNode(n)
		if c == nil {
			continue
		}
		res, err := transformNode(ctx.WithNodes(doc, c))
		if err != nil {
			return nil, ctx.errorWithContext(err)
		}
		seq.Concat(res)
	}
	return seq, nil
}

func executeResultDocument(ctx *Context) (xml.Sequence, error) {
	elem, err := getElementFromNode(ctx.XslNode)
	if err != nil {
		return nil, ctx.errorWithContext(err)
	}

	var doc xml.Document
	for _, n := range slices.Clone(elem.Nodes) {
		c := cloneNode(n)
		if c == nil {
			continue
		}
		seq, err := transformNode(ctx.WithXsl(c))
		if err != nil {
			return nil, err
		}
		for i := range seq {
			doc.Nodes = append(doc.Nodes, seq[i].Node())
		}
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

func executeVariable(ctx *Context) (xml.Sequence, error) {
	elem, err := getElementFromNode(ctx.XslNode)
	if err != nil {
		return nil, ctx.errorWithContext(err)
	}
	ident, err := getAttribute(elem, "name")
	if err != nil {
		return nil, ctx.errorWithContext(err)
	}
	var seq xml.Sequence
	if query, err1 := getAttribute(elem, "select"); err1 == nil {
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
	ctx.Define(ident, xml.NewValueFromSequence(seq))
	return nil, nil
}

func executeWithParam(ctx *Context) (xml.Sequence, error) {
	elem, err := getElementFromNode(ctx.XslNode)
	if err != nil {
		return nil, ctx.errorWithContext(err)
	}
	ident, err := getAttribute(elem, "name")
	if err != nil {
		return nil, ctx.errorWithContext(err)
	}
	if query, err := getAttribute(elem, "select"); err == nil {
		ctx.EvalParam(ident, query, ctx.ContextNode)
	} else {
		var seq xml.Sequence
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
		ctx.DefineExprParam(ident, xml.NewValueFromSequence(seq))
	}
	return nil, nil
}

func executeApplyImport(ctx *Context) (xml.Sequence, error) {
	return executeApply(ctx, ctx.MatchImport)
}

func executeApplyTemplates(ctx *Context) (xml.Sequence, error) {
	return executeApply(ctx, ctx.Match)
}

func executeCallTemplate(ctx *Context) (xml.Sequence, error) {
	elem, err := getElementFromNode(ctx.XslNode)
	if err != nil {
		return nil, ctx.errorWithContext(err)
	}
	name, err := getAttribute(elem, "name")
	if err != nil {
		return nil, ctx.errorWithContext(err)
	}
	mode, _ := getAttribute(elem, "mode")
	tpl, err := ctx.Find(name, mode)
	if err != nil {
		return ctx.NotFound(err, mode)
	}
	sub := tpl.mergeContext(ctx)
	if err := applyParams(sub); err != nil {
		return nil, ctx.errorWithContext(err)
	}
	seq := xml.NewSequence()
	for _, n := range slices.Clone(tpl.Nodes) {
		c := cloneNode(n)
		if c == nil {
			continue
		}
		res, err := transformNode(sub.WithXsl(c))
		if err != nil {
			return nil, ctx.errorWithContext(err)
		}
		seq.Concat(res)
	}
	return seq, nil
}

func executeForeachGroup(ctx *Context) (xml.Sequence, error) {
	elem, err := getElementFromNode(ctx.XslNode)
	if err != nil {
		return nil, ctx.errorWithContext(err)
	}
	query, err := getAttribute(elem, "select")
	if err != nil {
		return nil, err
	}

	items, err := ctx.ExecuteQuery(query, ctx.ContextNode)
	if err != nil || len(items) == 0 {
		return nil, err
	}

	key, err := getAttribute(elem, "group-by")
	if err != nil {
		return nil, err
	}
	grpby, err := ctx.CompileQuery(key)
	if err != nil {
		return nil, err
	}
	groups := make(map[string]xml.Sequence)
	for i := range items {
		is, err := grpby.Find(items[i].Node())
		if err != nil {
			return nil, err
		}
		key := is[0].Value().(string)
		groups[key] = append(groups[key], items[i])
	}

	seq := xml.NewSequence()
	for key, items := range groups {
		defineForeachGroupBuiltins(ctx, key, items)
		for _, n := range elem.Nodes {
			c := cloneNode(n)
			if c == nil {
				continue
			}
			res, err := transformNode(ctx.WithXsl(c))
			if err != nil {
				return nil, err
			}
			seq.Concat(res)
		}
	}
	return seq, nil
}

type MergedItem struct {
	xml.Item
	Key    string
	Source string
}

func executeMerge(ctx *Context) (xml.Sequence, error) {
	elem, err := getElementFromNode(ctx.XslNode)
	if err != nil {
		return nil, ctx.errorWithContext(err)
	}
	var (
		action xml.Node
		groups = make(map[string][]MergedItem)
	)

	for _, n := range elem.Nodes {
		if n.QualifiedName() != ctx.getQualifiedName("merge-source") {
			action = n
			break
		}
		el := n.(*xml.Element)
		ident, err := getAttribute(el, "name")
		if err != nil {
			return nil, err
		}
		var items xml.Sequence
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
		seq  = xml.NewSequence()
	)
	slices.Sort(keys)
	for _, key := range keys {
		nested := ctx.Nest()
		defineMergeBuiltins(nested, key, groups[key])
		res, err := appendNode(nested)
		if err != nil {
			return nil, err
		}
		seq.Concat(res)
	}
	return seq, nil
}

func executeForeach(ctx *Context) (xml.Sequence, error) {
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
		return nil, nil
	}
	it, err := applySort(ctx, items)
	if err != nil {
		return nil, ctx.errorWithContext(err)
	}

	seq := xml.NewSequence()
	for i := range it {
		node := i.Node()
		for _, n := range elem.Nodes {
			c := cloneNode(n)
			if c == nil {
				continue
			}
			res, err := transformNode(ctx.WithNodes(node, c))
			if err != nil {
				return nil, err
			}
			seq.Concat(res)
		}
	}
	return seq, nil
}

func executeTry(ctx *Context) (xml.Sequence, error) {
	items, err := ctx.queryXSL("./catch[last()]")
	if err != nil {
		return nil, err
	}
	if len(items) > 1 {
		return nil, fmt.Errorf("only one catch element is allowed")
	}
	seq, err := processNode(ctx)
	if err != nil {
		if len(items) > 0 {
			catch := items[0].Node()
			return processNode(ctx.WithXsl(catch))
		}
		return nil, err
	}
	return seq, nil
}

func executeIf(ctx *Context) (xml.Sequence, error) {
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
	if !ok {
		return nil, nil
	}
	return processNode(ctx)
}

func executeChoose(ctx *Context) (xml.Sequence, error) {
	items, err := ctx.queryXSL("/when")
	if err != nil {
		return nil, err
	}
	for i := range items {
		node, err := getElementFromNode(items[i].Node())
		if err != nil {
			return nil, ctx.errorWithContext(err)
		}
		test, err := getAttribute(node, "test")
		if err != nil {
			return nil, err
		}
		ok, err := ctx.TestNode(test, ctx.ContextNode)
		if err != nil {
			return nil, err
		}
		if ok {
			return processNode(ctx)
		}
	}

	if items, err = ctx.queryXSL("otherwise"); err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return nil, nil
	}
	return processNode(ctx.WithXsl(items[0].Node()))
}

func executeValueOf(ctx *Context) (xml.Sequence, error) {
	elem, err := getElementFromNode(ctx.XslNode)
	if err != nil {
		return nil, ctx.errorWithContext(err)
	}
	query, err := getAttribute(elem, "select")
	if err != nil {
		return nil, ctx.errorWithContext(err)
	}
	sep, err := getAttribute(elem, "separator")
	if err != nil {
		sep = " "
	}
	items, err := ctx.ExecuteQuery(query, ctx.ContextNode)
	if err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return nil, nil
	}

	var str strings.Builder
	for i := range items {
		if i > 0 {
			str.WriteString(sep)
		}
		str.WriteString(toString(items[i]))
	}
	return xml.Singleton(xml.NewText(str.String())), nil
}

func executeCopy(ctx *Context) (xml.Sequence, error) {
	return executeCopyOf(ctx)
}

func executeCopyOf(ctx *Context) (xml.Sequence, error) {
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
	seq := xml.NewSequence()
	for i := range items {
		c := cloneNode(items[i].Node())
		if c != nil {
			seq.Append(xml.NewNodeItem(c))
		}
	}
	return seq, nil
}

func executeMessage(ctx *Context) (xml.Sequence, error) {
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

func executeEvaluate(ctx *Context) (xml.Sequence, error) {
	return nil, errImplemented
}

func executeAnalyzeString(ctx *Context) (xml.Sequence, error) {
	return nil, errImplemented
}

func executeMatchingSubstring(ctx *Context) (xml.Sequence, error) {
	return nil, errImplemented
}

func executeNonMatchingSubstring(ctx *Context) (xml.Sequence, error) {
	return nil, errImplemented
}

func executeWherePopulated(ctx *Context) (xml.Sequence, error) {
	elem, err := getElementFromNode(ctx.XslNode)
	if err != nil {
		return nil, ctx.errorWithContext(err)
	}
	var (
		nodes = slices.Clone(elem.Nodes)
		seq   xml.Sequence
	)

	for _, n := range nodes {
		res, err := transformNode(ctx.WithXsl(n))
		if err != nil {
			return nil, err
		}
		seq.Concat(res)
	}
	if seq.Empty() {
		return nil, errEmpty
	}
	return seq, nil
}

func executeOnEmpty(ctx *Context) (xml.Sequence, error) {
	return nil, errImplemented
}

func executeOnNotEmpty(ctx *Context) (xml.Sequence, error) {
	return nil, errImplemented
}

func executeSequence(ctx *Context) (xml.Sequence, error) {
	elem, err := getElementFromNode(ctx.XslNode)
	if err != nil {
		return nil, ctx.errorWithContext(err)
	}
	query, err := getAttribute(elem, "select")
	if err != nil {
		return nil, err
	}
	return ctx.ExecuteQuery(query, ctx.ContextNode)
}

func executeElement(ctx *Context) (xml.Sequence, error) {
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
	curr := xml.NewElement(qn)
	for i := range elem.Nodes {
		c := cloneNode(elem.Nodes[i])
		if c == nil {
			continue
		}
		seq, err := transformNode(ctx.WithXsl(c))
		if err != nil {
			return nil, err
		}
		for i := range seq {
			curr.Nodes = append(curr.Nodes, seq[i].Node())
		}
	}
	return xml.Singleton(curr), nil
}

func executeAttribute(ctx *Context) (xml.Sequence, error) {
	return nil, errImplemented
}

func executeText(ctx *Context) (xml.Sequence, error) {
	elem := xml.NewText(ctx.XslNode.Value())
	return xml.Singleton(xml.NewNodeItem(elem)), nil
}

func executeComment(ctx *Context) (xml.Sequence, error) {
	elem := xml.NewComment(ctx.XslNode.Value())
	return xml.Singleton(xml.NewNodeItem(elem)), nil
}

func executeFallback(ctx *Context) (xml.Sequence, error) {
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

func applySort(ctx *Context, items []xml.Item) (iter.Seq[xml.Item], error) {
	sorts, err := ctx.queryXSL("./sort[1]")
	if err != nil {
		return nil, err
	}
	if len(sorts) == 0 {
		return slices.Values(items), nil
	}
	elem, err := getElementFromNode(sorts[0].Node())
	if err != nil {
		return nil, ctx.errorWithContext(err)
	}
	query, err := getAttribute(elem, "select")
	if err != nil {
		return nil, err
	}
	order, _ := getAttribute(elem, "order")
	return iterItems(items, query, order)
}

type matchFunc func(xml.Node, string) (*Template, error)

func executeApply(ctx *Context, match matchFunc) (xml.Sequence, error) {
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
	var (
		mode, _ = getAttribute(elem, "mode")
		seq     = xml.NewSequence()
	)
	for _, datum := range nodes {
		tpl, err := match(datum, mode)
		if err != nil {
			for i := range nodes {
				res, err := ctx.WithXpath(nodes[i]).NotFound(err, mode)
				if err != nil {
					return nil, err
				}
				seq.Concat(res)
			}
			return seq, nil
		}
		sub := tpl.mergeContext(ctx.WithXpath(datum))
		if err := applyParams(sub); err != nil {
			return nil, err
		}
		res, err := tpl.Execute(sub)
		if err != nil {
			return nil, err
		}
		for i := range res {
			seq.Append(xml.NewNodeItem(res[i]))
		}
	}
	return seq, nil
}

func iterItems(items []xml.Item, orderBy, orderDir string) (iter.Seq[xml.Item], error) {
	expr, err := xml.CompileString(orderBy)
	if err != nil {
		return nil, err
	}
	getString := func(is []xml.Item) string {
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
	fn := func(yield func(xml.Item) bool) {
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

func defineForeachGroupBuiltins(nested *Context, key string, seq xml.Sequence) {
	currentGrp := func(_ xml.Context, _ []xml.Expr) (xml.Sequence, error) {
		return seq, nil
	}
	currentKey := func(_ xml.Context, _ []xml.Expr) (xml.Sequence, error) {
		i := xml.NewLiteralItem(key)
		return []xml.Item{i}, nil
	}

	nested.Builtins.Define("current-group", currentGrp)
	nested.Builtins.Define("fn:current-group", currentGrp)
	nested.Builtins.Define("current-grouping-key", currentKey)
	nested.Builtins.Define("fn:current-grouping-key", currentKey)
}

func defineMergeBuiltins(nested *Context, key string, items []MergedItem) {
	currentKey := func(_ xml.Context, _ []xml.Expr) (xml.Sequence, error) {
		return xml.Singleton(key), nil
	}
	currentGrp := func(ctx xml.Context, args []xml.Expr) (xml.Sequence, error) {
		if len(args) > 1 {
			return nil, fmt.Errorf("too many arguments")
		}
		var (
			seq xml.Sequence
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
