package caches

import "testing"

func TestParseDockerSize(t *testing.T) {
	for in, want := range map[string]int64{
		"1.2GB (50%)": 1_200_000_000,
		"512kB":       512_000,
		"0B (0%)":     0,
		"3.4MB":       3_400_000,
		"":            0,
	} {
		if got := parseDockerSize(in); got != want {
			t.Errorf("%q: got %d want %d", in, got, want)
		}
	}
}
