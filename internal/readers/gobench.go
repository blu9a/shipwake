package readers

import (
	"bufio"
	"io"
	"strconv"
	"strings"

	"github.com/blu9a/shipwake/internal/model"
)

// readGoBench reads Go benchmark output. Each benchmark is a suite with its time per operation
// and, when reported, allocations per operation; repeated runs (-count) become repeated values.
func readGoBench(r io.Reader) (model.Run, error) {
	run := model.Run{Context: map[string]string{}}
	byID := map[string]int{}
	add := func(suite, id, name, unit string, v float64) {
		i, ok := byID[id]
		if !ok {
			i = len(run.Metrics)
			byID[id] = i
			run.Metrics = append(run.Metrics, model.Metric{ID: id, Suite: suite, Name: name, Unit: unit, Direction: "lower"})
		}
		run.Metrics[i].Values = append(run.Metrics[i].Values, v)
	}
	sc := bufio.NewScanner(r)
	for sc.Scan() {
		line := sc.Text()
		for _, k := range []string{"cpu", "goos", "goarch", "pkg"} {
			if v, ok := strings.CutPrefix(line, k+": "); ok && k != "pkg" {
				run.Context[k] = v
			}
		}
		f := strings.Fields(line)
		if len(f) < 4 || !strings.HasPrefix(f[0], "Benchmark") {
			continue
		}
		name := strings.TrimPrefix(f[0], "Benchmark")
		if i := strings.LastIndex(name, "-"); i > 0 { // drop the GOMAXPROCS suffix
			if _, err := strconv.Atoi(name[i+1:]); err == nil {
				name = name[:i]
			}
		}
		for j := 2; j+1 < len(f); j += 2 {
			v, err := strconv.ParseFloat(f[j], 64)
			if err != nil {
				continue
			}
			switch f[j+1] {
			case "ns/op":
				add(name, name+".time", "Time per operation", "ns", v)
			case "allocs/op":
				add(name, name+".allocs", "Allocations per operation", "allocs", v)
			case "B/op":
				add(name, name+".bytes", "Memory per operation", "B", v)
			}
		}
	}
	return run, sc.Err()
}
