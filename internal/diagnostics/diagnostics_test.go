package diagnostics

import "testing"

func TestParseCgroupLimit(t *testing.T) {
	cases := []struct {
		in   string
		want int64
		ok   bool
	}{
		{"4294967296\n", 4294967296, true},  // 4Gi (the incident's limit)
		{"max\n", 0, false},                 // cgroup v2 unlimited
		{"9223372036854771712\n", 0, false}, // cgroup v1 "unlimited" (PAGE_COUNTER_MAX)
		{"", 0, false},                      // empty
		{"garbage", 0, false},               // unparsable
		{"-1", 0, false},                    // negative
		{"1048576", 0, false},               // 1Mi — implausibly small, ignore
		{"536870912", 536870912, true},      // 512Mi
	}
	for _, c := range cases {
		got, ok := parseCgroupLimit(c.in)
		if ok != c.ok || (ok && got != c.want) {
			t.Errorf("parseCgroupLimit(%q) = (%d, %v), want (%d, %v)", c.in, got, ok, c.want, c.ok)
		}
	}
}
