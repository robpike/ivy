// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package exec

import (
	"fmt"

	"robpike.io/ivy/value"
)

// Info returns information about the value. It is in package
// exec so it has access to the Function type.
func Info(c value.Context, v value.Value) []string {
	typ := value.TypeOf(v)
	s := ""
	switch typ { // Could use a type switch but this is a little prettier....
	case "int", "char", "big int", "rational", "float", "complex": // ... because of this case.
		s = typ
	case "operator":
		return operatorInfo(c, v.(*value.OpExpr))
	case "vector":
		v := v.(*value.Vector)
		s = fmt.Sprintf("%s vector of length %d", v.ElementTypes(c), v.Len())
	case "matrix":
		m := v.(*value.Matrix)
		s = fmt.Sprintf("%s matrix of rank %d, size %d, dimension %d", m.Data().ElementTypes(c), m.Rank(), m.Size(c), m.Shape())
	default:
		c.Errorf("unknown type %T in Info", typ)
	}
	return []string{s}
}

// indent adds the prefix to each line and returns a new slice.
func indent(prefix string, lines []string) []string {
	nLines := make([]string, len(lines))
	for i, line := range lines {
		nLines[i] = prefix + line
	}
	return nLines
}

// operatorInfo is like Info but applies to operators instead of values.
func operatorInfo(c value.Context, op value.Operator) (lines []string) {
	if op == nil {
		return nil
	}

	add := func(s string) { lines = append(lines, s) }

	addInner := func(c value.Context, op value.Operator) {
		lines = append(lines, indent("\t", operatorInfo(c, op))...)
	}

	addInitializers := func(inits []*value.Var) {
		if len(inits) == 0 {
			return
		}
		add("initializers:")
		for _, init := range inits {
			val := init.Value()
			add(fmt.Sprintf("\t%s = %s", init.Name(), val))
			lines = append(lines, indent("\t\t", Info(c, val))...)
		}
	}

	switch op := op.(type) {
	default:
		add(fmt.Sprintf("unimplemented info for type %T", op))
	case *value.OpExpr:
		unary, binary := op.Unpack()
		lines = append(lines, operatorInfo(c, unary)...)
		lines = append(lines, operatorInfo(c, binary)...)
	case *value.ReduceOp:
		kind := "reduce"
		if op.IsScan {
			kind = "scan"
		}
		axis := ""
		if op.FirstAxis {
			axis = " on first axis"
		}
		add(fmt.Sprintf("unary %s operator%s of:", kind, axis))
		addInner(c, op.Op)
	case *value.InnerOp:
		add("binary inner product operator of:")
		addInner(c, op.LeftOp)
		add("and")
		addInner(c, op.RightOp)
	case *value.OuterOp:
		add("binary outer product operator of:")
		addInner(c, op.Op)
	case *Function:
		if op.IsBinary {
			add("binary user-defined operator")
		} else {
			add("unary user-defined operator")
		}
	case value.UnaryBuiltinOp:
		add("unary builtin operator")
	case *value.UnaryEachOp:
		add(fmt.Sprintf("unary each operator (%d iterations) of:", op.NumAts))
		addInner(c, op.Op)
	case *value.UnaryNonceOp:
		add("unary nonce operator")
		addInitializers(op.Initializers())
	case value.BinaryBuiltinOp:
		add("binary builtin operator")
	case *value.BinaryEachOp:
		var left, right string
		if op.LeftAts > 0 {
			left = fmt.Sprintf(" (%d left iterations)", op.LeftAts)
		}
		if op.RightAts > 0 {
			right = fmt.Sprintf(" (%d right iterations)", op.RightAts)
		}
		add(fmt.Sprintf("binary each operator%s%s of: ", left, right))
		addInner(c, op.Op)
	case *value.BinaryNonceOp:
		add("binary nonce operator")
		addInitializers(op.Initializers())
	}
	return lines
}
