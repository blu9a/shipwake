package readers

import (
	"encoding/json"
	"io"

	"github.com/blu9a/shipwake/internal/model"
)

// readOwn reads Shipwake's own run format (see `shipwake schema`, "run"): the way to bring in anything
// without a reader, such as an eval runner's scores. The ID is set when recording; a commit or branch in the
// file is kept unless the command names another.
func readOwn(r io.Reader) (model.Run, error) {
	var run model.Run
	if err := json.NewDecoder(r).Decode(&run); err != nil {
		return run, err
	}
	if err := run.CheckSchema(); err != nil {
		return run, err
	}
	for i := range run.Metrics {
		if run.Metrics[i].Direction == "" {
			run.Metrics[i].Direction = "lower"
		}
		run.Metrics[i].Previous, run.Metrics[i].Release = nil, nil
	}
	if run.Kind == "" {
		run.Kind = "test"
		if len(run.Metrics) > 0 {
			run.Kind = "measurement"
		}
	}
	finish(&run)
	return run, nil
}
