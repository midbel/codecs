package xslt

import (
	"fmt"
	"maps"
	"slices"
	"strings"
	"path/filepath"
	"iter"
	"os"
	"sort"

	"github.com/midbel/codecs/xml"
)

type ExecuteFunc func(*Context, xml.Node) (xml.Sequence, error)

var executers map[xml.QName]ExecuteFunc

func init() {
	wrap := func(exec ExecuteFunc) ExecuteFunc {
		fn := func(ctx *Context, node xml.Node) (xml.Sequence, error) {
			return exec(ctx.Self(), node)
		}
		return fn
	}
	executers = map[xml.QName]ExecuteFunc{
		xml.QualifiedName("for-each", xsltNamespacePrefix):        wrap(executeForeach),
		xml.QualifiedName("value-of", xsltNamespacePrefix):        executeValueOf,
		xml.QualifiedName("call-template", xsltNamespacePrefix):   wrap(executeCallTemplate),
		xml.QualifiedName("apply-templates", xsltNamespacePrefix): wrap(executeApplyTemplates),
		xml.QualifiedName("apply-imports", xsltNamespacePrefix):   wrap(executeApplyImport),
		xml.QualifiedName("if", xsltNamespacePrefix):              wrap(executeIf),
		xml.QualifiedName("choose", xsltNamespacePrefix):          wrap(executeChoose),
		xml.QualifiedName("where-populated", xsltNamespacePrefix): executeWherePopulated,
		xml.QualifiedName("on-empty", xsltNamespacePrefix):        executeOnEmpty,
		xml.QualifiedName("on-not-empty", xsltNamespacePrefix):    executeOnNotEmpty,
		xml.QualifiedName("try", xsltNamespacePrefix):             wrap(executeTry),
		xml.QualifiedName("variable", xsltNamespacePrefix):        executeVariable,
		xml.QualifiedName("result-document", xsltNamespacePrefix): executeResultDocument,
		xml.QualifiedName("source-document", xsltNamespacePrefix): executeSourceDocument,
		xml.QualifiedName("import", xsltNamespacePrefix):          executeImport,
		xml.QualifiedName("include", xsltNamespacePrefix):         executeInclude,
		xml.QualifiedName("with-param", xsltNamespacePrefix):      executeWithParam,
		xml.QualifiedName("copy", xsltNamespacePrefix):            executeCopy,
		xml.QualifiedName("copy-of", xsltNamespacePrefix):         executeCopyOf,
		xml.QualifiedName("sequence", xsltNamespacePrefix):        executeSequence,
		xml.QualifiedName("element", xsltNamespacePrefix):         executeElement,
		xml.QualifiedName("attribute", xsltNamespacePrefix):       executeAttribute,
		xml.QualifiedName("text", xsltNamespacePrefix):            executeText,
		xml.QualifiedName("comment", xsltNamespacePrefix):         executeComment,
		xml.QualifiedName("message", xsltNamespacePrefix):         executeMessage,
		xml.QualifiedName("fallback", xsltNamespacePrefix):        executeFallback,
		xml.QualifiedName("merge", xsltNamespacePrefix):           executeMerge,
		xml.QualifiedName("for-each-group", xsltNamespacePrefix):  executeForeachGroup,
	}
}

func executeImport(ctx *Context, node xml.Node) (xml.Sequence, error) {
	el := node.(*xml.Element)
	file, err := getAttribute(el, "href")
	if err != nil {
		return nil, err
	}
	return nil, ctx.ImportSheet(file)
}

func executeInclude(ctx *Context, node xml.Node) (xml.Sequence, error) {
	el := node.(*xml.Element)
	file, err := getAttribute(el, "href")
	if err != nil {
		return nil, err
	}
	return nil, ctx.IncludeSheet(file)
}

func executeSourceDocument(ctx *Context, node xml.Node) (xml.Sequence, error) {
	el := node.(*xml.Element)
	file, err := getAttribute(el, "href")
	if err != nil {
		return nil, err
	}
	doc, err := loadDocument(filepath.Join(ctx.Context, file))
	if err != nil {
		return nil, err
	}
	var (
		nodes []xml.Node
		sub   = ctx.Sub(doc)
	)
	for _, n := range slices.Clone(el.Nodes) {
		c := cloneNode(n)
		if c == nil {
			continue
		}
		if _, err := transformNode(sub, c); err != nil {
			return nil, err
		}
		nodes = append(nodes, c)
	}
	return nil, insertNodes(el, nodes...)
}

func executeResultDocument(ctx *Context, node xml.Node) (xml.Sequence, error) {
	el := node.(*xml.Element)

	var doc xml.Document
	for _, n := range slices.Clone(el.Nodes) {
		c := cloneNode(n)
		if c == nil {
			continue
		}
		if _, err := transformNode(ctx, c); err != nil {
			return nil, err
		}
		doc.Nodes = append(doc.Nodes, c)
	}

	file, err := getAttribute(el, "href")
	if err != nil {
		return nil, err
	}
	format, _ := getAttribute(el, "format")
	if err := writeDocument(file, format, &doc, ctx.Stylesheet); err != nil {
		return nil, err
	}
	if err := removeSelf(node); err != nil {
		return nil, err
	}
	return nil, errSkip
}

func executeVariable(ctx *Context, node xml.Node) (xml.Sequence, error) {
	el := node.(*xml.Element)
	ident, err := getAttribute(el, "name")
	if err != nil {
		return nil, err
	}
	if value, err := getAttribute(el, "select"); err == nil {
		query, err := ctx.Compile(value)
		if err != nil {
			return nil, err
		}
		ctx.Define(ident, query)
	} else {
		var res xml.Sequence
		for _, n := range slices.Clone(el.Nodes) {
			c := cloneNode(n)
			if c == nil {
				continue
			}
			seq, err := transformNode(ctx, c)
			if err != nil {
				return nil, err
			}
			res = slices.Concat(res, seq)
		}
		ctx.Define(ident, xml.NewValueFromSequence(res))
	}
	return nil, removeSelf(node)
}

func executeWithParam(ctx *Context, node xml.Node) (xml.Sequence, error) {
	el := node.(*xml.Element)
	ident, err := getAttribute(el, "name")
	if err != nil {
		return nil, err
	}
	if query, err := getAttribute(el, "select"); err == nil {
		ctx.EvalParam(ident, query, ctx.CurrentNode)
	} else {
		var res xml.Sequence
		for _, n := range slices.Clone(el.Nodes) {
			c := cloneNode(n)
			if c == nil {
				continue
			}
			seq, err := transformNode(ctx, c)
			if err != nil {
				return nil, err
			}
			res = slices.Concat(res, seq)
		}
		ctx.DefineExprParam(ident, xml.NewValueFromSequence(res))
	}
	return nil, removeSelf(node)
}

func executeApplyImport(ctx *Context, node xml.Node) (xml.Sequence, error) {
	return executeApply(ctx, node, ctx.MatchImport)
}

func executeApplyTemplates(ctx *Context, node xml.Node) (xml.Sequence, error) {
	return executeApply(ctx, node, ctx.Match)
}

func executeCallTemplate(ctx *Context, node xml.Node) (xml.Sequence, error) {
	el := node.(*xml.Element)
	name, err := getAttribute(el, "name")
	if err != nil {
		return nil, err
	}
	mode, _ := getAttribute(el, "mode")
	tpl, err := ctx.Find(name, mode)
	if err != nil {
		return nil, ctx.NotFound(node, err, mode)
	}
	sub := tpl.createContext(ctx, ctx.CurrentNode)
	if err := applyParams(sub, node); err != nil {
		return nil, err
	}
	parent := node.Parent().(*xml.Element)
	for _, n := range slices.Clone(tpl.Nodes) {
		c := cloneNode(n)
		if c == nil {
			continue
		}
		parent.Append(c)
		if _, err := transformNode(sub, c); err != nil {
			return nil, err
		}
	}
	return nil, removeSelf(node)
}

func executeForeachGroup(ctx *Context, node xml.Node) (xml.Sequence, error) {
	el := node.(*xml.Element)
	query, err := getAttribute(el, "select")
	if err != nil {
		return nil, err
	}
	parent, ok := el.Parent().(*xml.Element)
	if !ok {
		return nil, fmt.Errorf("for-each-group: xml element expected as parent")
	}

	items, err := ctx.Execute(query, ctx.CurrentNode)
	if err != nil || len(items) == 0 {
		return nil, err
	}

	key, err := getAttribute(el, "group-by")
	if err != nil {
		return nil, err
	}
	grpby, err := ctx.Compile(key)
	if err != nil {
		return nil, err
	}
	groups := make(map[string][]xml.Item)
	for i := range items {
		is, err := grpby.Find(items[i].Node())
		if err != nil {
			return nil, err
		}
		key := is[0].Value().(string)
		groups[key] = append(groups[key], items[i])
	}

	for key, items := range groups {
		currentGrp := func(_ xml.Context, _ []xml.Expr) (xml.Sequence, error) {
			return items, nil
		}
		currentKey := func(_ xml.Context, _ []xml.Expr) (xml.Sequence, error) {
			i := xml.NewLiteralItem(key)
			return []xml.Item{i}, nil
		}
		ctx.Builtins.Define("current-group", currentGrp)
		ctx.Builtins.Define("fn:current-group", currentGrp)
		ctx.Builtins.Define("current-grouping-key", currentKey)
		ctx.Builtins.Define("fn:current-grouping-key", currentKey)

		for _, n := range el.Nodes {
			c := cloneNode(n)
			if c == nil {
				continue
			}
			parent.Append(c)
			if _, err := transformNode(ctx, c); err != nil {
				return nil, err
			}
		}
	}
	return nil, removeSelf(node)
}

func executeMerge(ctx *Context, node xml.Node) (xml.Sequence, error) {
	type MergeItem struct {
		xml.Item
		Key    string
		Source string
	}
	var (
		elem   = node.(*xml.Element)
		action xml.Node
		groups = make(map[string][]MergeItem)
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
			items, err = ctx.Execute(query, ctx.CurrentNode)
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
			grp, err := ctx.Compile(query)
			if err != nil {
				return nil, err
			}
			for i := range items {
				is, err := grp.Find(items[i].Node())
				if err != nil {
					return nil, err
				}
				mit := MergeItem{
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

	keys := slices.Collect(maps.Keys(groups))
	slices.Sort(keys)
	for _, key := range keys {
		ctx := ctx.Self()

		items := groups[key]
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
					return nil, fmt.Errorf("no group avaialble")
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
		ctx.Builtins.Define("current-merge-group", currentGrp)
		ctx.Builtins.Define("fn:current-merge-group", currentGrp)
		ctx.Builtins.Define("current-merge-key", currentKey)
		ctx.Builtins.Define("fn:current-merge-key", currentKey)

		if err := appendNode(ctx, node); err != nil {
			return nil, err
		}
	}
	return nil, removeSelf(node)
}

func executeForeach(ctx *Context, node xml.Node) (xml.Sequence, error) {
	el := node.(*xml.Element)
	query, err := getAttribute(el, "select")
	if err != nil {
		return nil, err
	}

	items, err := ctx.Execute(query, ctx.CurrentNode)
	if err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return nil, removeSelf(node)
	}
	it, err := applySort(ctx, node, items)
	if err != nil {
		return nil, err
	}

	parent, ok := node.Parent().(*xml.Element)
	if !ok {
		return nil, fmt.Errorf("for-each: xml element expected as parent")
	}
	for i := range it {
		value := i.Node()
		for _, n := range el.Nodes {
			c := cloneNode(n)
			if c == nil {
				continue
			}
			parent.Append(c)
			if _, err := transformNode(ctx.Sub(value), c); err != nil {
				return nil, err
			}
		}
	}
	return nil, removeSelf(node)
}

func executeTry(ctx *Context, node xml.Node) (xml.Sequence, error) {
	el := node.(*xml.Element)
	items, err := ctx.Query("./catch[last()]", node)
	if err != nil {
		return nil, err
	}
	if len(items) > 1 {
		return nil, fmt.Errorf("only one catch element is allowed")
	}
	if _, err := processNode(ctx, el); err != nil {
		if len(items) > 0 {
			catch := items[0].Node()
			if err := removeNode(node, catch); err != nil {
				return nil, err
			}
			return processNode(ctx.Self(), catch)
		}
		return nil, err
	}
	return nil, nil
}

func executeIf(ctx *Context, node xml.Node) (xml.Sequence, error) {
	el := node.(*xml.Element)
	test, err := getAttribute(el, "test")
	if err != nil {
		return nil, err
	}
	ok, err := ctx.Test(test, ctx.CurrentNode)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, removeSelf(el)
	}
	if _, err = processNode(ctx, node); err != nil {
		return nil, err
	}
	return nil, insertNodes(el, el.Nodes...)
}

func executeChoose(ctx *Context, node xml.Node) (xml.Sequence, error) {
	items, err := ctx.Query("/when", ctx.CurrentNode)
	if err != nil {
		return nil, err
	}
	for i := range items {
		n := items[i].Node().(*xml.Element)
		test, err := getAttribute(n, "test")
		if err != nil {
			return nil, err
		}
		ok, err := ctx.Test(test, ctx.CurrentNode)
		if err != nil {
			return nil, err
		}
		if ok {
			if _, err := processNode(ctx, n); err != nil {
				return nil, err
			}
			var (
				pt = n.Parent()
				gp = pt.Parent()
			)
			if i, ok := gp.(interface{ InsertNodes(int, []xml.Node) error }); ok {
				return nil, i.InsertNodes(pt.Position(), n.Nodes)
			}
			return nil, nil
		}
	}

	if items, err = ctx.Query("otherwise", node); err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return nil, nil
	}
	n := items[0].Node().(*xml.Element)
	if _, err := processNode(ctx, n); err != nil {
		return nil, err
	}
	var (
		pt = n.Parent()
		gp = pt.Parent()
	)
	if i, ok := gp.(interface{ InsertNodes(int, []xml.Node) error }); ok {
		return nil, i.InsertNodes(pt.Position(), n.Nodes)
	}
	return nil, nil
}

func executeValueOf(ctx *Context, node xml.Node) (xml.Sequence, error) {
	el := node.(*xml.Element)
	query, err := getAttribute(el, "select")
	if err != nil {
		return nil, err
	}
	sep, err := getAttribute(el, "separator")
	if err != nil {
		sep = " "
	}
	items, err := ctx.Execute(query, ctx.CurrentNode)
	if err != nil || len(items) == 0 {
		return nil, removeSelf(node)
	}

	var str strings.Builder
	for i := range items {
		if i > 0 {
			str.WriteString(sep)
		}
		str.WriteString(toString(items[i]))
	}
	text := xml.NewText(str.String())
	return nil, replaceNode(node, text)
}

func executeCopy(ctx *Context, node xml.Node) (xml.Sequence, error) {
	return executeCopyOf(ctx, node)
}

func executeCopyOf(ctx *Context, node xml.Node) (xml.Sequence, error) {
	el := node.(*xml.Element)
	query, err := getAttribute(el, "select")
	if err != nil {
		return nil, err
	}
	items, err := ctx.Execute(query, ctx.CurrentNode)
	if err != nil {
		return nil, err
	}
	var list []xml.Node
	for i := range items {
		c := cloneNode(items[i].Node())
		if c != nil {
			list = append(list, c)
		}
	}
	return nil, insertNodes(el, list...)
}

func executeMessage(ctx *Context, node xml.Node) (xml.Sequence, error) {
	var (
		parts []string
		el    = node.(*xml.Element)
	)
	for _, n := range el.Nodes {
		parts = append(parts, n.Value())
	}
	fmt.Fprintln(os.Stderr, strings.Join(parts, ""))

	if quit, err := getAttribute(el, "terminate"); err == nil && quit == "yes" {
		return nil, ErrTerminate
	}
	return nil, nil
}

func executeWherePopulated(ctx *Context, node xml.Node) (xml.Sequence, error) {
	return nil, errImplemented
}

func executeOnEmpty(ctx *Context, node xml.Node) (xml.Sequence, error) {
	return nil, errImplemented
}

func executeOnNotEmpty(ctx *Context, node xml.Node) (xml.Sequence, error) {
	return nil, errImplemented
}

func executeSequence(ctx *Context, node xml.Node) (xml.Sequence, error) {
	elem := node.(*xml.Element)
	query, err := getAttribute(elem, "select")
	if err != nil {
		return nil, err
	}
	return ctx.Execute(query, ctx.CurrentNode)
}

func executeElement(ctx *Context, node xml.Node) (xml.Sequence, error) {
	el := node.(*xml.Element)
	ident, err := getAttribute(el, "name")
	if err != nil {
		return nil, err
	}
	qn, err := xml.ParseName(ident)
	if err != nil {
		return nil, err
	}
	var (
		curr  = xml.NewElement(qn)
		nodes = slices.Clone(el.Nodes)
	)
	if err := replaceNode(el, curr); err != nil {
		return nil, err
	}
	for i := range nodes {
		c := cloneNode(nodes[i])
		if c == nil {
			continue
		}
		curr.Append(c)
		if _, err := transformNode(ctx, c); err != nil {
			return nil, err
		}
	}
	return nil, nil
}

func executeAttribute(ctx *Context, node xml.Node) (xml.Sequence, error) {
	return nil, errImplemented
}

func executeText(ctx *Context, node xml.Node) (xml.Sequence, error) {
	text := xml.NewText(node.Value())
	return nil, replaceNode(node, text)
}

func executeComment(ctx *Context, node xml.Node) (xml.Sequence, error) {
	comment := xml.NewComment(node.Value())
	return nil, replaceNode(node, comment)
}

func executeFallback(ctx *Context, node xml.Node) (xml.Sequence, error) {
	return nil, errImplemented
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

func getNodesForTemplate(ctx *Context, node xml.Node) ([]xml.Node, error) {
	var (
		el  = node.(*xml.Element)
		res []xml.Node
	)
	if query, err := getAttribute(el, "select"); err == nil {
		items, err := ctx.Execute(query, ctx.CurrentNode)
		if err != nil {
			return nil, err
		}
		for i := range items {
			res = append(res, items[i].Node())
		}
	} else {
		res = []xml.Node{cloneNode(ctx.CurrentNode)}
	}
	return res, nil
}

func applyParams(ctx *Context, node xml.Node) error {
	el := node.(*xml.Element)
	for _, n := range slices.Clone(el.Nodes) {
		if n.QualifiedName() != ctx.getQualifiedName("with-param") {
			return fmt.Errorf("%s: invalid child node %s", node.QualifiedName(), n.QualifiedName())
		}
		if _, err := transformNode(ctx, n); err != nil {
			return err
		}
	}
	return nil
}

func applySort(ctx *Context, node xml.Node, items []xml.Item) (iter.Seq[xml.Item], error) {
	sorts, err := ctx.Query("./sort[1]", node)
	if err != nil {
		return nil, err
	}
	if len(sorts) == 0 {
		return slices.Values(items), nil
	}
	tmp := sorts[0].Node()
	if err := removeSelf(tmp); err != nil {
		return nil, err
	}
	elem, ok := tmp.(*xml.Element)
	if !ok {
		return nil, fmt.Errorf("sort: expected xml element")
	}
	query, err := getAttribute(elem, "select")
	if err != nil {
		return nil, err
	}
	order, _ := getAttribute(elem, "order")
	return iterItems(items, query, order)
}

type matchFunc func(xml.Node, string) (*Template, error)

func executeApply(ctx *Context, node xml.Node, match matchFunc) (xml.Sequence, error) {
	nodes, err := getNodesForTemplate(ctx, node)
	if err != nil {
		return nil, err
	}
	if len(nodes) == 0 {
		return nil, removeNode(node, node)
	}
	var (
		el      = node.(*xml.Element)
		mode, _ = getAttribute(el, "mode")
		results []xml.Node
	)
	for _, datum := range nodes {
		tpl, err := match(datum, mode)
		if err != nil {
			for i := range nodes {
				sub := ctx.Sub(nodes[i])
				if err = sub.NotFound(node, err, mode); err != nil {
					return nil, err
				}
			}
			return nil, err
		}
		sub := tpl.createContext(ctx, datum)
		if err := applyParams(sub, node); err != nil {
			return nil, err
		}
		res, err := tpl.Execute(sub)
		if err != nil {
			return nil, err
		}
		results = slices.Concat(results, res)
	}
	return nil, insertNodes(node, results...)
}
