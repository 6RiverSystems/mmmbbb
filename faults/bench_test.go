package faults

import "testing"

func Benchmark_Check_empty_miss_nil_params(b *testing.B) {
	s := NewSet()
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		if s.Check("op", nil) != nil {
			b.Fatal("check of empty set returned an error")
		}
	}
}

func Benchmark_Check_empty_miss_small_params(b *testing.B) {
	s := NewSet()
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		if s.Check("op", Parameters{"op": "foo"}) != nil {
			b.Fatal("check of empty set returned an error")
		}
	}
}

// allocating and freeing a map per call is _expensive_
func Benchmark_Check_empty_miss_small_params_pre(b *testing.B) {
	s := NewSet()
	p := Parameters{"op": "foo"}
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		if s.Check("op", p) != nil {
			b.Fatal("check of empty set returned an error")
		}
	}
}

func Benchmark_Check_filled_miss_nil_params(b *testing.B) {
	s := NewSet()
	s.Add(Description{"op1", nil, nil, 1})
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		if s.Check("op2", nil) != nil {
			b.Fatal("check of empty set returned an error")
		}
	}
}

func Benchmark_Check_filled_miss_pval_small_params_pre(b *testing.B) {
	s := NewSet()
	s.Add(Description{"op", Parameters{"a": "1"}, nil, 1})
	p := Parameters{"a": "2"}
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		if s.Check("op", p) != nil {
			b.Fatal("check of empty set returned an error")
		}
	}
}
func Benchmark_Check_filled_miss_pname_small_params_pre(b *testing.B) {
	s := NewSet()
	s.Add(Description{"op", Parameters{"a": "1"}, nil, 1})
	p := Parameters{"b": "1"}
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		if s.Check("op", p) != nil {
			b.Fatal("check of empty set returned an error")
		}
	}
}
