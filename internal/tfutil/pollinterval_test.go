package tfutil

import (
	"testing"
	"time"
)

func TestPollInterval(t *testing.T) {
	const def = 15 * time.Second
	cases := []struct {
		name string
		env  string
		set  bool
		want time.Duration
	}{
		{name: "unset falls back", set: false, want: def},
		{name: "empty falls back", env: "", set: true, want: def},
		{name: "valid override", env: "2s", set: true, want: 2 * time.Second},
		{name: "malformed falls back", env: "soon", set: true, want: def},
		{name: "zero falls back", env: "0s", set: true, want: def},
		{name: "negative falls back", env: "-3s", set: true, want: def},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.set {
				t.Setenv(PollIntervalEnv, tc.env)
			}
			if got := PollInterval(def); got != tc.want {
				t.Errorf("PollInterval(%s) = %s, want %s", def, got, tc.want)
			}
		})
	}
}
