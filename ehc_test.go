package ehc

import (
	"testing"
	"time"
)

func TestEHC_Values(t *testing.T) {
	tests := []struct {
		name string
		ehc  func() *EHC
		want map[string]int64
	}{
		{
			name: "counts one key",
			ehc: func() *EHC {
				e := NewEHC(10 * time.Millisecond)
				e.Count("test")
				return e
			},
			want: map[string]int64{
				"test": 1,
			},
		},
		{
			name: "counts two of the same key",
			ehc: func() *EHC {
				e := NewEHC(10 * time.Millisecond)
				e.Count("test")
				e.Count("test")
				return e
			},
			want: map[string]int64{
				"test": 2,
			},
		},
		{
			name: "counts two of different key",
			ehc: func() *EHC {
				e := NewEHC(10 * time.Millisecond)
				e.Count("test1")
				e.Count("test2")
				return e
			},
			want: map[string]int64{
				"test1": 1,
				"test2": 1,
			},
		},
		{
			name: "expires a count",
			ehc: func() *EHC {
				e := NewEHC(10 * time.Millisecond)
				e.Count("test")
				time.Sleep(15 * time.Millisecond)
				return e
			},
			want: map[string]int64{},
		},
		{
			name: "retains an incremental count",
			ehc: func() *EHC {
				e := NewEHC(10 * time.Millisecond)
				e.Count("test")
				time.Sleep(7 * time.Millisecond)
				e.Count("test")
				return e
			},
			want: map[string]int64{
				"test": 2,
			},
		},
		{
			name: "expires a count incrementally",
			ehc: func() *EHC {
				e := NewEHC(10 * time.Millisecond)
				e.Count("test")
				time.Sleep(7 * time.Millisecond)
				e.Count("test")
				time.Sleep(5 * time.Millisecond)
				return e
			},
			want: map[string]int64{
				"test": 1,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, locker := tt.ehc().Values()
			defer locker.Unlock()
			if len(got) != len(tt.want) {
				t.Errorf("EHC.Values() unexpected number of values, %d != %d", len(got), len(tt.want))
			}
			for k, v := range tt.want {
				if got[k].Value() != v {
					t.Errorf("EHC.Values() got = %d, want %d", got[k].Value(), v)
				}
			}
		})
	}
}
