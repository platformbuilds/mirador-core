//go:build ignore

package main

import (
	"fmt"

	"github.com/grindlemire/go-lucene"
	"github.com/grindlemire/go-lucene/pkg/lucene/expr"
)

func printAST(e *expr.Expression, indent int) {
	indentStr := ""
	for i := 0; i < indent; i++ {
		indentStr += "  "
	}

	fmt.Printf("%sOp: %v\n", indentStr, e.Op)
	if e.Left != nil {
		fmt.Printf("%sLeft:\n", indentStr)
		if leftExpr, ok := e.Left.(*expr.Expression); ok {
			printAST(leftExpr, indent+1)
		} else {
			fmt.Printf("%s  %T: %+v\n", indentStr, e.Left, e.Left)
		}
	}
	if e.Right != nil {
		fmt.Printf("%sRight:\n", indentStr)
		if rightExpr, ok := e.Right.(*expr.Expression); ok {
			printAST(rightExpr, indent+1)
		} else {
			fmt.Printf("%s  %T: %+v\n", indentStr, e.Right, e.Right)
		}
	}
}

func main() {
	testQueries := []string{
		"error AND timeout",
		"error OR timeout",
		"duration:[100 TO 500]",
		"error*",
		"level:error",
		"_time:[now-1d TO now]",
		`level:error AND (message:"timeout" OR message:"failed")`,
		"level:err*",
	}

	for _, q := range testQueries {
		parsed, err := lucene.Parse(q)
		if err != nil {
			fmt.Printf("Error parsing '%s': %v\n", q, err)
		} else {
			fmt.Printf("Query: '%s'\n", q)
			printAST(parsed, 0)
			fmt.Println()
		}
	}
}
