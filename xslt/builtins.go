package xslt

import (
	"fmt"

	"github.com/midbel/codecs/xpath"
)

func callCurrent(ctx xpath.Context, _ []xpath.Expr) (xpath.Sequence, error) {
	return xpath.Singleton(ctx.Node), nil
}

func callId(ctx xpath.Context, _ []xpath.Expr) (xpath.Sequence, error) {
	return nil, nil
}

func callKey(ctx xpath.Context, _ []xpath.Expr) (xpath.Sequence, error) {
	return nil, nil
}

func callDocument(ctx xpath.Context, _ []xpath.Expr) (xpath.Sequence, error) {
	return nil, nil
}

func callSystemProperty(ctx xpath.Context, args []xpath.Expr) (xpath.Sequence, error) {
	if len(args) > 1 {
		return nil, fmt.Errorf("invalid number of arguments")
	}
	items, err := args[0].Find(ctx)
	if err != nil {
		return nil, err
	}
	if items.Empty() {
		return items, nil
	}
	str, ok := items[0].Value().(string)
	if !ok {
		return nil, nil
	}
	switch str {
	case "xsl:version":
		str = XslVersion
	case "xsl:vendor":
		str = XslVendor
	case "xsl:vendor-url":
		str = XslVendorUrl
	case "product-name":
		str = XslProduct
	case "xsl:product-version":
		str = XslProductVersion
	default:
		return nil, fmt.Errorf("%s: unknown system property", str)
	}
	return xpath.Singleton(str), nil
}
