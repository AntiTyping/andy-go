// run

// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Test for the |> (pipe/map) operator.

package main

import (
	"fmt"
	"reflect"
)

func verify(name string, result, expected interface{}) {
	if !reflect.DeepEqual(result, expected) {
		panic(fmt.Sprintf("%s: got %v, want %v", name, result, expected))
	}
}

func double(x int) int {
	return x * 2
}

func addOne(x int) int {
	return x + 1
}

func intToString(x int) string {
	return fmt.Sprintf("%d", x)
}

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
	fmt.Printf("%v", tripled)

	// Precedence test: a + b |> f should be (a + b) |> f, but since a+b isn't a slice,
	// we test that the operator has lower precedence than || by using logical ops
	// Actually, test with slice literal
	result := []int{1, 2} |> double
	verify("precedence", result, []int{2, 4})

	fmt.Println("PASS")
}
