# Implementing the `|>` Pipe Operator in the Go Compiler

This document provides an in-depth explanation of how the `|>` (pipe) operator was implemented in the Go compiler. The pipe operator applies a function to each element of a slice and returns a new slice with the mapped values.

## Table of Contents

1. [Overview](#overview)
2. [Operator Semantics](#operator-semantics)
3. [Compiler Pipeline](#compiler-pipeline)
4. [Implementation Details](#implementation-details)
   - [Step 1: Token Definition](#step-1-token-definition)
   - [Step 2: Scanner Recognition](#step-2-scanner-recognition)
   - [Step 3: IR Operation Definition](#step-3-ir-operation-definition)
   - [Step 4: Syntax to IR Mapping](#step-4-syntax-to-ir-mapping)
   - [Step 5: Types2 Type Checking](#step-5-types2-type-checking)
   - [Step 6: IR Type Checking](#step-6-ir-type-checking)
   - [Step 7: Escape Analysis](#step-7-escape-analysis)
   - [Step 8: Walk Phase Transformation](#step-8-walk-phase-transformation)
5. [Error Handling](#error-handling)
6. [Testing](#testing)
7. [Files Modified](#files-modified)

---

## Overview

The pipe operator `|>` is a binary operator that takes a slice on the left and a function on the right, returning a new slice where each element is the result of applying the function to the corresponding element of the input slice.

```go
numbers := []int{1, 2, 3, 4, 5}
doubled := numbers |> func(x int) int { return x * 2 }
// doubled = []int{2, 4, 6, 8, 10}
```

This is equivalent to the functional programming concept of `map`, but expressed as an infix operator.

---

## Operator Semantics

### Type Signature

```
[]T |> func(T) U → []U
```

- **Left operand**: Must be a slice of type `[]T`
- **Right operand**: Must be a function of type `func(T) U` with exactly one parameter and one result
- **Result**: A new slice of type `[]U`

### Precedence

The pipe operator has the **lowest precedence** (level 1), even lower than `||` (logical OR). This means:

```go
a || b |> f    // Parses as: (a || b) |> f
slice |> f |> g // Parses as: (slice |> f) |> g (left-to-right chaining)
```

### Associativity

Left-to-right, enabling natural chaining:

```go
numbers |> double |> addOne |> square
// Equivalent to: ((numbers |> double) |> addOne) |> square
```

---

## Compiler Pipeline

The Go compiler processes source code through several phases. The pipe operator required modifications at each stage:

```
Source Code
    ↓
┌─────────────────────────────────────────────────────────┐
│ 1. LEXER (Scanner)                                      │
│    Tokenizes "|>" into a Pipe token                     │
│    File: syntax/scanner.go                              │
└─────────────────────────────────────────────────────────┘
    ↓
┌─────────────────────────────────────────────────────────┐
│ 2. PARSER                                               │
│    Recognizes Pipe as a binary operator with precPipe   │
│    File: syntax/tokens.go                               │
└─────────────────────────────────────────────────────────┘
    ↓
┌─────────────────────────────────────────────────────────┐
│ 3. TYPE CHECKER (types2)                                │
│    Validates operand types before IR generation         │
│    File: types2/expr.go                                 │
└─────────────────────────────────────────────────────────┘
    ↓
┌─────────────────────────────────────────────────────────┐
│ 4. NODER                                                │
│    Converts syntax tree to IR, maps Pipe → OPIPE        │
│    Files: noder/noder.go, noder/writer.go               │
└─────────────────────────────────────────────────────────┘
    ↓
┌─────────────────────────────────────────────────────────┐
│ 5. IR TYPE CHECKER                                      │
│    Re-validates types at IR level                       │
│    Files: typecheck/typecheck.go, typecheck/expr.go     │
└─────────────────────────────────────────────────────────┘
    ↓
┌─────────────────────────────────────────────────────────┐
│ 6. ESCAPE ANALYSIS                                      │
│    Determines if allocations escape to heap             │
│    File: escape/expr.go                                 │
└─────────────────────────────────────────────────────────┘
    ↓
┌─────────────────────────────────────────────────────────┐
│ 7. WALK PHASE                                           │
│    Transforms OPIPE into a for loop                     │
│    Files: walk/expr.go, walk/builtin.go                 │
└─────────────────────────────────────────────────────────┘
    ↓
┌─────────────────────────────────────────────────────────┐
│ 8. SSA GENERATION & CODE GEN                            │
│    (No changes needed - works with transformed IR)      │
└─────────────────────────────────────────────────────────┘
```

---

## Implementation Details

### Step 1: Token Definition

**File: `src/cmd/compile/internal/syntax/tokens.go`**

First, we define the `Pipe` operator constant and its precedence level:

```go
// Operator precedence levels (lowest to highest)
const (
    _ = iota
    precPipe    // 1 - lowest (pipe operator)
    precOrOr    // 2
    precAndAnd  // 3
    precCmp     // 4
    precAdd     // 5
    precMul     // 6 - highest
)

// Operator constants
const (
    // ... existing operators ...

    // precPipe
    Pipe  // |>

    // precOrOr
    OrOr  // ||

    // ... rest of operators ...
)
```

The key insight here is that `precPipe` is defined as 1, making it the lowest precedence operator. This ensures expressions like `a + b |> f` are parsed as `(a + b) |> f`.

### Step 2: Scanner Recognition

**File: `src/cmd/compile/internal/syntax/scanner.go`**

The scanner (lexer) must recognize the two-character sequence `|>` as a single token:

```go
case '|':
    s.nextch()
    if s.ch == '|' {
        s.nextch()
        s.op, s.prec = OrOr, precOrOr
        s.tok = _Operator
        break
    }
    if s.ch == '>' {
        s.nextch()
        s.op, s.prec = Pipe, precPipe
        s.tok = _Operator
        break
    }
    s.op, s.prec = Or, precAdd
    goto assignop
```

When the scanner sees `|`, it looks ahead:
- If the next character is `|`, it's the `||` operator
- If the next character is `>`, it's the `|>` operator
- Otherwise, it's the bitwise OR `|` operator

### Step 3: IR Operation Definition

**File: `src/cmd/compile/internal/ir/node.go`**

The IR (Intermediate Representation) needs an operation constant for the pipe operator:

```go
const (
    // ... existing operations ...
    OOROR             // X || Y
    OPIPE             // X |> Y (pipe/map operator)
    OPANIC            // panic(X)
    // ... rest of operations ...
)
```

**File: `src/cmd/compile/internal/ir/expr.go`**

The `BinaryExpr` type needs to allow `OPIPE` as a valid operation:

```go
func (n *BinaryExpr) SetOp(op Op) {
    switch op {
    default:
        panic(n.no("SetOp " + op.String()))
    case OADD, OADDSTR, OAND, OANDNOT, ODIV, OEQ, OGE, OGT, OLE,
        OLSH, OLT, OMOD, OMUL, ONE, OOR, ORSH, OSUB, OXOR,
        OCOPY, OCOMPLEX, OUNSAFEADD, OUNSAFESLICE, OUNSAFESTRING,
        OMAKEFACE, OPIPE:  // OPIPE added here
        n.op = op
    }
}
```

**File: `src/cmd/compile/internal/ir/fmt.go`**

For debugging and error messages, add the operator to the name and precedence maps:

```go
var opNames = [...]string{
    // ... existing entries ...
    OPIPE: "|>",
}

var opPrec = [...]int{
    // ... existing entries ...
    OPIPE: 0,  // Lowest precedence
}
```

### Step 4: Syntax to IR Mapping

**File: `src/cmd/compile/internal/noder/noder.go`**

Map the syntax-level `Pipe` operator to the IR-level `OPIPE`:

```go
var binOps = [...]ir.Op{
    syntax.Pipe:   ir.OPIPE,
    syntax.OrOr:   ir.OOROR,
    syntax.AndAnd: ir.OANDAND,
    // ... rest of mappings ...
}
```

**File: `src/cmd/compile/internal/noder/writer.go`**

The noder's writer phase handles binary operations by finding a common type between operands. The pipe operator is special because its operands have incompatible types (slice and function):

```go
var commonType types2.Type
switch expr.Op {
case syntax.Shl, syntax.Shr:
    // ok: operands are allowed to have different types
case syntax.Pipe:
    // Pipe operator: left is slice, right is function - no common type
    w.Code(exprBinaryOp)
    w.op(binOps[expr.Op])
    w.expr(expr.X)
    w.pos(expr)
    w.expr(expr.Y)
    break
default:
    // ... normal common type handling ...
}
```

### Step 5: Types2 Type Checking

**File: `src/cmd/compile/internal/types2/expr.go`**

The `types2` package is Go's type checker that runs before IR generation. We add a dispatch for pipe operations:

```go
// In the binary expression handling:
if op == syntax.Pipe {
    check.pipe(x, &y, e)
    return
}
```

And implement the `pipe` method:

```go
// pipe type checks the pipe operator: slice |> func(T) U
// The result type is []U.
func (check *Checker) pipe(x, y *operand, e syntax.Expr) {
    // Avoid spurious errors if any of the operands has an invalid type.
    if !isValid(x.typ) || !isValid(y.typ) {
        x.mode = invalid
        return
    }

    // Left operand must be a slice
    sliceType, ok := coreType(x.typ).(*Slice)
    if !ok {
        check.errorf(x, InvalidPipe,
            invalidOp+"%s |> %s (left operand must be a slice, got %s)",
            x.expr, y.expr, x.typ)
        x.mode = invalid
        return
    }

    // Right operand must be a function
    sig, ok := coreType(y.typ).(*Signature)
    if !ok {
        check.errorf(y, InvalidPipe,
            invalidOp+"%s |> %s (right operand must be a function, got %s)",
            x.expr, y.expr, y.typ)
        x.mode = invalid
        return
    }

    // Function must have exactly 1 parameter
    if sig.Params().Len() != 1 {
        check.errorf(y, InvalidPipe,
            invalidOp+"%s |> %s (function must have exactly 1 parameter, got %d)",
            x.expr, y.expr, sig.Params().Len())
        x.mode = invalid
        return
    }

    // Function must have exactly 1 result
    if sig.Results().Len() != 1 {
        check.errorf(y, InvalidPipe,
            invalidOp+"%s |> %s (function must have exactly 1 result, got %d)",
            x.expr, y.expr, sig.Results().Len())
        x.mode = invalid
        return
    }

    elemType := sliceType.Elem()
    paramType := sig.Params().At(0).Type()
    resultType := sig.Results().At(0).Type()

    // Parameter type must match slice element type
    if !Identical(elemType, paramType) {
        check.errorf(y, InvalidPipe,
            invalidOp+"%s |> %s (cannot use func(%s) with []%s)",
            x.expr, y.expr, paramType, elemType)
        x.mode = invalid
        return
    }

    // Result type is []resultType
    x.mode = value
    x.typ = NewSlice(resultType)
}
```

**File: `src/internal/types/errors/codes.go`**

Add an error code for pipe-related errors:

```go
// InvalidPipe occurs when the pipe operator |> is used incorrectly.
// The left operand must be a slice, and the right operand must be
// a function with exactly one parameter matching the slice element type
// and exactly one result.
//
// Example:
//  var _ = 42 |> func(x int) int { return x * 2 }
InvalidPipe
```

### Step 6: IR Type Checking

**File: `src/cmd/compile/internal/typecheck/typecheck.go`**

Add dispatch for `OPIPE` in the main type checking switch:

```go
case ir.OPIPE:
    n := n.(*ir.BinaryExpr)
    return tcPipe(n)
```

**File: `src/cmd/compile/internal/typecheck/expr.go`**

Implement the IR-level type checker for pipe:

```go
// tcPipe type checks a pipe expression: slice |> func(T) U
func tcPipe(n *ir.BinaryExpr) ir.Node {
    n.X = Expr(n.X)
    n.Y = Expr(n.Y)

    l := n.X
    r := n.Y

    if l.Type() == nil || r.Type() == nil {
        n.SetType(nil)
        return n
    }

    // Left operand must be a slice
    if !l.Type().IsSlice() {
        base.Errorf("invalid operation: %v (left operand must be a slice, got %v)",
            n, l.Type())
        n.SetType(nil)
        return n
    }

    // Right operand must be a function
    if r.Type().Kind() != types.TFUNC {
        base.Errorf("invalid operation: %v (right operand must be a function, got %v)",
            n, r.Type())
        n.SetType(nil)
        return n
    }

    fn := r.Type()

    // Function must have exactly 1 parameter
    if fn.NumParams() != 1 {
        base.Errorf("invalid operation: %v (function must have exactly 1 parameter, got %d)",
            n, fn.NumParams())
        n.SetType(nil)
        return n
    }

    // Function must have exactly 1 result
    if fn.NumResults() != 1 {
        base.Errorf("invalid operation: %v (function must have exactly 1 result, got %d)",
            n, fn.NumResults())
        n.SetType(nil)
        return n
    }

    elemType := l.Type().Elem()
    paramType := fn.Param(0).Type
    resultType := fn.Result(0).Type

    // Parameter type must match slice element type
    if !types.Identical(elemType, paramType) {
        base.Errorf("invalid operation: %v (cannot use func(%v) with []%v)",
            n, paramType, elemType)
        n.SetType(nil)
        return n
    }

    // Result type is []resultType
    n.SetType(types.NewSlice(resultType))
    return n
}
```

### Step 7: Escape Analysis

**File: `src/cmd/compile/internal/escape/expr.go`**

Escape analysis determines whether allocations can stay on the stack or must escape to the heap. Add handling for `OPIPE`:

```go
case ir.OPIPE:
    // Pipe operator: slice |> func -> new slice
    // Elements of input slice flow through the function.
    // The function may be a closure capturing variables.
    // The result is a new slice that contains the function results.
    n := n.(*ir.BinaryExpr)
    e.expr(k.note(n, "pipe input slice"), n.X)
    e.expr(k.note(n, "pipe function"), n.Y)
```

### Step 8: Walk Phase Transformation

The walk phase is where the magic happens. The `OPIPE` operation is transformed into equivalent Go code using a for loop.

**File: `src/cmd/compile/internal/walk/expr.go`**

Add dispatch for `OPIPE`:

```go
case ir.OPIPE:
    n := n.(*ir.BinaryExpr)
    return walkPipe(n, init)
```

**File: `src/cmd/compile/internal/walk/builtin.go`**

Implement the transformation. The expression `slice |> fn` is transformed into:

```go
// Original:
result := slice |> fn

// Transformed to:
{
    s := slice          // Evaluate slice once
    f := fn             // Evaluate function once
    result := make([]U, len(s))
    for i := 0; i < len(s); i++ {
        result[i] = f(s[i])
    }
}
// result is the final value
```

Here's the implementation:

```go
// walkPipe transforms: slice |> fn
// Into:
//
//  init {
//    s := slice
//    f := fn
//    result := make([]U, len(s))
//    for i := 0; i < len(s); i++ {
//      result[i] = f(s[i])
//    }
//  }
//  result
func walkPipe(n *ir.BinaryExpr, init *ir.Nodes) ir.Node {
    n.X = walkExpr(n.X, init)
    n.Y = walkExpr(n.Y, init)

    slice := n.X
    fn := n.Y
    resultType := n.Type() // []U

    // Create temp for slice to avoid multiple evaluation
    s := typecheck.TempAt(base.Pos, ir.CurFunc, slice.Type())
    as := ir.NewAssignStmt(base.Pos, s, slice)
    init.Append(typecheck.Stmt(as))

    // Create temp for function to avoid multiple evaluation
    f := typecheck.TempAt(base.Pos, ir.CurFunc, fn.Type())
    af := ir.NewAssignStmt(base.Pos, f, fn)
    init.Append(typecheck.Stmt(af))

    // Create result slice: result := make([]U, len(s))
    result := typecheck.TempAt(base.Pos, ir.CurFunc, resultType)
    lenExpr := ir.NewUnaryExpr(base.Pos, ir.OLEN, s)
    lenExpr = typecheck.Expr(lenExpr).(*ir.UnaryExpr)
    lenExpr = walkExpr(lenExpr, init).(*ir.UnaryExpr)

    makeCall := ir.NewMakeExpr(base.Pos, ir.OMAKESLICE, nil, nil)
    makeCall.SetType(resultType)
    makeCall.Len = lenExpr
    makeCall.Cap = lenExpr
    makeCall.SetTypecheck(1)
    walkedMake := walkExpr(makeCall, init)
    ar := ir.NewAssignStmt(base.Pos, result, walkedMake)
    init.Append(typecheck.Stmt(ar))

    // Build the for loop: for i := 0; i < len(s); i++ { result[i] = f(s[i]) }
    i := typecheck.TempAt(base.Pos, ir.CurFunc, types.Types[types.TINT])
    hn := typecheck.TempAt(base.Pos, ir.CurFunc, types.Types[types.TINT])

    // Initialize loop variables
    ai := ir.NewAssignStmt(base.Pos, i, ir.NewInt(base.Pos, 0))
    init.Append(typecheck.Stmt(ai))

    lenS := ir.NewUnaryExpr(base.Pos, ir.OLEN, s)
    lenS = typecheck.Expr(lenS).(*ir.UnaryExpr)
    lenS = walkExpr(lenS, init).(*ir.UnaryExpr)
    aln := ir.NewAssignStmt(base.Pos, hn, lenS)
    init.Append(typecheck.Stmt(aln))

    // Build loop body: result[i] = f(s[i])
    var body []ir.Node

    // s[i]
    srcIndex := ir.NewIndexExpr(base.Pos, s, i)
    srcIndex.SetBounded(true)
    srcIndex = typecheck.Expr(srcIndex).(*ir.IndexExpr)

    // f(s[i])
    call := ir.NewCallExpr(base.Pos, ir.OCALL, f, []ir.Node{srcIndex})
    call = typecheck.Expr(call).(*ir.CallExpr)

    // result[i]
    dstIndex := ir.NewIndexExpr(base.Pos, result, i)
    dstIndex.SetBounded(true)
    dstIndex = typecheck.Expr(dstIndex).(*ir.IndexExpr)

    // result[i] = f(s[i])
    assign := ir.NewAssignStmt(base.Pos, dstIndex, call)
    assign = typecheck.Stmt(assign).(*ir.AssignStmt)
    body = append(body, assign)

    // Build the for statement condition: i < hn
    cond := ir.NewBinaryExpr(base.Pos, ir.OLT, i, hn)
    cond = typecheck.Expr(cond).(*ir.BinaryExpr)

    // Build the for statement post: i++
    incr := ir.NewBinaryExpr(base.Pos, ir.OADD, i, ir.NewInt(base.Pos, 1))
    post := ir.NewAssignStmt(base.Pos, i, incr)
    post = typecheck.Stmt(post).(*ir.AssignStmt)

    nfor := ir.NewForStmt(base.Pos, nil, cond, post, body, false)
    appendWalkStmt(init, nfor)

    return result
}
```

Key implementation details:

1. **Temporary variables**: We store `slice` and `fn` in temporaries to ensure they're only evaluated once
2. **Bounded indexing**: We set `SetBounded(true)` on index expressions since we know `i` is always in bounds
3. **Proper type checking**: Every IR node we create is type-checked before use
4. **Walking expressions**: We walk the len expressions and make expression to transform them to their final forms

---

## Error Handling

The implementation provides clear error messages for various misuse cases:

```go
// Left operand must be a slice
x := 42
_ = x |> func(y int) int { return y * 2 }
// Error: invalid operation: x |> func(y int) int {…} (left operand must be a slice, got int)

// Right operand must be a function
slice := []int{1, 2, 3}
x := 42
_ = slice |> x
// Error: invalid operation: slice |> x (right operand must be a function, got int)

// Function must have exactly 1 parameter
_ = slice |> func(a, b int) int { return a + b }
// Error: invalid operation: slice |> func(a, b int) int {…} (function must have exactly 1 parameter, got 2)

// Parameter type must match slice element type
_ = slice |> func(s string) string { return s + "!" }
// Error: invalid operation: slice |> func(s string) string {…} (cannot use func(string) with []int)
```

---

## Testing

**File: `test/pipe.go`**

The implementation includes comprehensive tests:

```go
func main() {
    // Basic usage
    numbers := []int{1, 2, 3, 4, 5}
    doubled := numbers |> double
    verify("basic double", doubled, []int{2, 4, 6, 8, 10})

    // Empty slice
    empty := []int{} |> double
    verify("empty slice", empty, []int{})

    // Single element
    single := []int{42} |> double
    verify("single element", single, []int{84})

    // Chaining
    chained := numbers |> double |> addOne
    verify("chaining", chained, []int{3, 5, 7, 9, 11})

    // Type transformation
    strings := numbers |> intToString
    verify("type transform", strings, []string{"1", "2", "3", "4", "5"})

    // Function literal
    tripled := numbers |> func(x int) int { return x * 3 }
    verify("func literal", tripled, []int{3, 6, 9, 12, 15})

    fmt.Println("PASS")
}
```

---

## Files Modified

| File | Purpose |
|------|---------|
| `src/cmd/compile/internal/syntax/tokens.go` | Token and precedence definitions |
| `src/cmd/compile/internal/syntax/scanner.go` | Lexer recognition of `\|>` |
| `src/cmd/compile/internal/syntax/operator_string.go` | Generated stringer |
| `src/cmd/compile/internal/ir/node.go` | IR operation constant |
| `src/cmd/compile/internal/ir/op_string.go` | Generated stringer |
| `src/cmd/compile/internal/ir/fmt.go` | Debug formatting |
| `src/cmd/compile/internal/ir/expr.go` | BinaryExpr SetOp allowlist |
| `src/cmd/compile/internal/noder/noder.go` | Syntax→IR mapping |
| `src/cmd/compile/internal/noder/writer.go` | Special handling for heterogeneous types |
| `src/cmd/compile/internal/types2/expr.go` | Pre-IR type checking |
| `src/cmd/compile/internal/typecheck/typecheck.go` | IR type check dispatch |
| `src/cmd/compile/internal/typecheck/expr.go` | IR type checking implementation |
| `src/cmd/compile/internal/escape/expr.go` | Escape analysis |
| `src/cmd/compile/internal/walk/expr.go` | Walk phase dispatch |
| `src/cmd/compile/internal/walk/builtin.go` | Walk transformation |
| `src/internal/types/errors/codes.go` | Error code definition |
| `test/pipe.go` | Runtime tests |

---

## Conclusion

Implementing a new operator in Go requires touching many parts of the compiler:

1. **Lexer**: Recognize the token
2. **Parser**: Handle precedence and associativity
3. **Type checker (types2)**: Validate types before IR
4. **Noder**: Convert syntax to IR
5. **Type checker (typecheck)**: Re-validate at IR level
6. **Escape analysis**: Track allocations
7. **Walk**: Transform to lower-level operations

The pipe operator is particularly interesting because:
- It has heterogeneous operand types (slice and function)
- It transforms to a for loop rather than a single operation
- It creates new allocations (the result slice)

This implementation provides a foundation for functional-style programming in Go while maintaining the language's emphasis on clarity and performance.
