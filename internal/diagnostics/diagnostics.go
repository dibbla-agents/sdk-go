package diagnostics

import (
	"log"
	"net/http"
	"net/http/pprof"
	"os"
	"runtime/debug"
	"strconv"
	"strings"
)

// Setup enables optional runtime diagnostics for the worker process:
//
//   - If SDK_PPROF_ADDR is set (e.g. "127.0.0.1:6060"), an HTTP server
//     exposing /debug/pprof/ is started on that address. Bind to localhost or
//     a cluster-internal address only; the endpoint is unauthenticated.
//   - If GOMEMLIMIT is not already set, the Go soft memory limit is derived
//     from the container's cgroup memory limit (90% of it), so the GC pushes
//     back before the kernel OOM-kills the process.
//
// Both are best-effort and never fail startup.
func Setup() {
	setupPprof()
	setupMemLimit()
}

func setupPprof() {
	addr := os.Getenv("SDK_PPROF_ADDR")
	if addr == "" {
		return
	}

	// Dedicated mux: never expose whatever else might be registered on
	// http.DefaultServeMux.
	mux := http.NewServeMux()
	mux.HandleFunc("/debug/pprof/", pprof.Index)
	mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
	mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	mux.HandleFunc("/debug/pprof/trace", pprof.Trace)

	go func() {
		log.Printf("pprof listening on http://%s/debug/pprof/", addr)
		if err := http.ListenAndServe(addr, mux); err != nil {
			log.Printf("pprof server error: %v", err)
		}
	}()
}

// setupMemLimit sets GOMEMLIMIT to 90% of the cgroup memory limit when the
// process runs in a memory-limited container and GOMEMLIMIT is not already
// set. No-op outside Linux/cgroups or when no limit is configured.
func setupMemLimit() {
	if os.Getenv("GOMEMLIMIT") != "" {
		return // explicit setting wins
	}

	limit, ok := cgroupMemoryLimit()
	if !ok {
		return
	}

	soft := limit * 9 / 10
	debug.SetMemoryLimit(soft)
	log.Printf("GOMEMLIMIT set to %d MiB (90%% of %d MiB cgroup limit)", soft>>20, limit>>20)
}

// cgroupMemoryLimit reads the container memory limit from cgroup v2 or v1.
// Returns ok=false when unlimited, implausible, or not in a cgroup.
func cgroupMemoryLimit() (int64, bool) {
	// cgroup v2
	if b, err := os.ReadFile("/sys/fs/cgroup/memory.max"); err == nil {
		return parseCgroupLimit(string(b))
	}
	// cgroup v1
	if b, err := os.ReadFile("/sys/fs/cgroup/memory/memory.limit_in_bytes"); err == nil {
		return parseCgroupLimit(string(b))
	}
	return 0, false
}

func parseCgroupLimit(s string) (int64, bool) {
	s = strings.TrimSpace(s)
	if s == "" || s == "max" {
		return 0, false // no limit configured
	}
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil || n <= 0 {
		return 0, false
	}
	// cgroup v1 reports "no limit" as a huge number (PAGE_COUNTER_MAX);
	// anything >= 1 EiB is not a real limit. Ignore tiny values too.
	if n >= 1<<60 || n < 16<<20 {
		return 0, false
	}
	return n, true
}
