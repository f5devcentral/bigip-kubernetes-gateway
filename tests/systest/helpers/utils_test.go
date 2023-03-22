package helpers

import "testing"

func Test_deepequal(t *testing.T) {
	type args struct {
		a interface{}
		b interface{}
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "string type",
			args: args{
				a: "string",
				b: "string",
			},
			want: true,
		},
		{
			name: "int type",
			args: args{
				a: 11,
				b: 0xb,
			},
			want: true,
		},
		{
			name: "slice type",
			args: args{
				a: []int{1, 2, 3},
				b: []int{3, 2, 1},
			},
			want: false,
		},
		{
			name: "interface type",
			args: args{
				a: []int{1, 2, 3},
				b: []interface{}{1, 2, 3},
			},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := deepequal(tt.args.a, tt.args.b); got != tt.want {
				t.Errorf("deepequal() = %v, want %v", got, tt.want)
			}
		})
	}
}
