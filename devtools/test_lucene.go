//go:build ignore

package main

import (
	"fmt"

	"github.com/grindlemire/go-lucene"
	"github.com/grindlemire/go-lucene/pkg/lucene/expr"
)

func main() {
	query := "_time:[now-1d TO now] AND service:otelgen"

	q, err := lucene.Parse(query)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	fmt.Printf("Top level Op: %v (type: %T)\n", q.Op, q.Op)

	if leftExpr, ok := q.Left.(*expr.Expression); ok {
		fmt.Printf("Left Op: %v (type: %T)\n", leftExpr.Op, leftExpr.Op)
	}

	if rightExpr, ok := q.Right.(*expr.Expression); ok {
		fmt.Printf("Right Op: %v (type: %T)\n", rightExpr.Op, rightExpr.Op)
	}
}
