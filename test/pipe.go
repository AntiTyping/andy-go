// run

// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Test pipe operator |>

package main

import "fmt"

func double(x int) int {
	return x * 2
}

func toString(x int) string {
	return fmt.Sprintf("%d", x)
}

func main() {
	// Basic usage
	nums := []int{1, 2, 3, 4, 5}
	doubled := nums |> double
	fmt.Printf("%v %T", doubled, doubled)
	if len(doubled) != 5 {
		panic("wrong length")
	}
	if doubled[0] != 2 || doubled[1] != 4 || doubled[2] != 6 || doubled[3] != 8 || doubled[4] != 10 {
		panic("basic pipe failed")
	}

	// Type transformation
	strings := nums |> toString
	if strings[0] != "1" || strings[4] != "5" {
		panic("type transformation failed")
	}

	// Chaining
	result := []int{1, 2, 3} |> double |> double
	if result[0] != 4 || result[1] != 8 || result[2] != 12 {
		panic("chaining failed")
	}

	// Empty slice
	empty := []int{} |> double
	if len(empty) != 0 {
		panic("empty slice failed")
	}

	// Anonymous function
	squared := nums |> func(x int) int { return x * x }
	if squared[0] != 1 || squared[1] != 4 || squared[2] != 9 {
		panic("anonymous function failed")
	}

	println("PASS")
}
