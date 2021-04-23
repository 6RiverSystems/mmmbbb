package faults

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDescription_match(t *testing.T) {
	type fields struct {
		Operation  string
		Parameters Parameters
		Count      int64
	}
	type args struct {
		op     string
		params Parameters
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   bool
	}{
		{
			"op only match",
			fields{
				Operation: "op1",
				Count:     1,
			},
			args{op: "op1"},
			true,
		},
		{
			"op only mismatch",
			fields{
				Operation: "op1",
				Count:     1,
			},
			args{op: "op2"},
			false,
		},
		{
			"params subset match",
			fields{"op", Parameters{"a": "1", "b": "2"}, 1},
			args{"op", Parameters{"a": "1", "b": "2", "c": "3"}},
			true,
		},
		{
			"params subset value mismatch",
			fields{"op", Parameters{"a": "1", "b": "2"}, 1},
			args{"op", Parameters{"a": "1", "b": "02", "c": "3"}},
			false,
		},
		{
			"params subset key mismatch",
			fields{"op", Parameters{"a": "1", "b": "2"}, 1},
			args{"op", Parameters{"a": "1", "bb": "2", "c": "3"}},
			false,
		},
		{
			"zero count",
			fields{Operation: "op"},
			args{op: "op"},
			false,
		},
		{
			"negative count",
			fields{Operation: "op", Count: -1},
			args{op: "op"},
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &Description{
				Operation:  tt.fields.Operation,
				Parameters: tt.fields.Parameters,
				Count:      tt.fields.Count,
			}
			assert.Equal(t, tt.want, d.match(tt.args.op, tt.args.params))
		})
	}
}
