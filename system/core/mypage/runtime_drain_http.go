package mypage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
)

func encodeRuntimeJSON(w http.ResponseWriter, status int, value any) error {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		return fmt.Errorf("encode runtime response: %w", err)
	}
	return nil
}

// The request ticket stays active through serialization and the actual Write,
// after the library has returned its token/snapshot. A rejected old result is
// never handed to the writer. A failed write has an uncertain delivery outcome.
func writeRuntimeResponse(ctx context.Context, w http.ResponseWriter, requestID string, write func() error) {
	work, workFound := ctx.Value(runtimeContextKey{}).(*runtimeWork)
	if !workFound {
		work = nil
	}
	err := work.fence(ctx, func() error {
		err := write()
		if err == nil && work != nil {
			// net/http buffers Write; flush before relinquishing delivery ownership.
			if flushErr := http.NewResponseController(w).Flush(); flushErr != nil {
				err = fmt.Errorf("flush runtime response: %w", flushErr)
			}
		}
		if err != nil {
			work.markUnknown()
		}
		return err
	})
	if errors.Is(err, ErrRuntimeFenced) {
		writeAPIError(w, 503, "TEMPORARY_UNAVAILABLE", requestID)
	}
}
