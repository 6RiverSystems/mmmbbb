// Copyright (c) 2021 6 River Systems
//
// Permission is hereby granted, free of charge, to any person obtaining a copy of
// this software and associated documentation files (the "Software"), to deal in
// the Software without restriction, including without limitation the rights to
// use, copy, modify, merge, publish, distribute, sublicense, and/or sell copies of
// the Software, and to permit persons to whom the Software is furnished to do so,
// subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY, FITNESS
// FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE AUTHORS OR
// COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY, WHETHER
// IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM, OUT OF OR IN
// CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.

package faults

import "testing"

func Benchmark_Check_empty_miss_nil_params(b *testing.B) {
	s := NewSet(b.Name())
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		if s.Check("op", nil) != nil {
			b.Fatal("check of empty set returned an error")
		}
	}
}

func Benchmark_Check_empty_miss_small_params(b *testing.B) {
	s := NewSet(b.Name())
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		if s.Check("op", Parameters{"op": "foo"}) != nil {
			b.Fatal("check of empty set returned an error")
		}
	}
}

// allocating and freeing a map per call is _expensive_
func Benchmark_Check_empty_miss_small_params_pre(b *testing.B) {
	s := NewSet(b.Name())
	p := Parameters{"op": "foo"}
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		if s.Check("op", p) != nil {
			b.Fatal("check of empty set returned an error")
		}
	}
}

func Benchmark_Check_filled_miss_nil_params(b *testing.B) {
	s := NewSet(b.Name())
	s.Add(Description{"op1", nil, nil, 1, "grpc.NotFound"})
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		if s.Check("op2", nil) != nil {
			b.Fatal("check of empty set returned an error")
		}
	}
}

func Benchmark_Check_filled_miss_pval_small_params_pre(b *testing.B) {
	s := NewSet(b.Name())
	s.Add(Description{"op", Parameters{"a": "1"}, nil, 1, "grpc.NotFound"})
	p := Parameters{"a": "2"}
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		if s.Check("op", p) != nil {
			b.Fatal("check of empty set returned an error")
		}
	}
}

func Benchmark_Check_filled_miss_pname_small_params_pre(b *testing.B) {
	s := NewSet(b.Name())
	s.Add(Description{"op", Parameters{"a": "1"}, nil, 1, "grpc.NotFound"})
	p := Parameters{"b": "1"}
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		if s.Check("op", p) != nil {
			b.Fatal("check of empty set returned an error")
		}
	}
}
