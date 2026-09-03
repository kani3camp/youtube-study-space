//go:build formalspec

package repository

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"
)

const (
	seatConformanceFileEnv = "FSL_SEAT_CONFORMANCE_FILE"
	seatModelClockMax      = 4
)

var seatModelEpoch = time.Date(2026, 1, 1, 12, 0, 0, 0, time.FixedZone("JST", 9*60*60))

type seatConformanceDocument struct {
	SchemaVersion        string                  `json:"schema_version"`
	KernelSchemaVersion  string                  `json:"kernel_schema_version"`
	Result               string                  `json:"result"`
	Spec                 string                  `json:"spec"`
	States               []seatConformanceState  `json:"states"`
	Vectors              []seatConformanceVector `json:"vectors"`
}

type seatConformanceState struct {
	ID    string       `json:"id"`
	State seatFSLState `json:"state"`
}

type seatConformanceVector struct {
	State   string                 `json:"state"`
	Action  seatConformanceAction  `json:"action"`
	Outcome seatConformanceOutcome `json:"outcome"`
}

type seatConformanceAction struct {
	Name   string         `json:"name"`
	Params map[string]int `json:"params"`
}

type seatConformanceOutcome struct {
	Kind         string       `json:"kind"`
	StateChanged bool         `json:"state_changed"`
	State        seatFSLState `json:"state"`
}

type seatFSLState struct {
	Mode                    string `json:"mode"`
	Now                     int    `json:"now"`
	Until                   int    `json:"until"`
	CurrentStateStartedAt   int    `json:"current_state_started_at"`
	CurrentStateUntil       int    `json:"current_state_until"`
	CurrentSegmentStartedAt int    `json:"current_segment_started_at"`
	CumulativeWork          int    `json:"cumulative_work"`
}

func TestSeatFSLConformance(t *testing.T) {
	vectorPath := os.Getenv(seatConformanceFileEnv)
	if vectorPath == "" {
		t.Fatalf("%s is required when running formalspec tests", seatConformanceFileEnv)
	}

	data, err := os.ReadFile(vectorPath)
	if err != nil {
		t.Fatalf("read conformance vectors: %v", err)
	}

	var doc seatConformanceDocument
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatalf("decode conformance vectors: %v", err)
	}
	if !strings.HasPrefix(doc.SchemaVersion, "1.") {
		t.Fatalf("unsupported conformance schema version: %q", doc.SchemaVersion)
	}
	if !strings.HasPrefix(doc.KernelSchemaVersion, "1.") {
		t.Fatalf("unsupported kernel schema version: %q", doc.KernelSchemaVersion)
	}
	if doc.Result != "conformance" {
		t.Fatalf("unexpected conformance result: %q", doc.Result)
	}
	if doc.Spec != "SeatSession" {
		t.Fatalf("unexpected spec: %q", doc.Spec)
	}
	if len(doc.Vectors) == 0 {
		t.Fatal("conformance vector set is empty")
	}

	states := make(map[string]seatFSLState, len(doc.States))
	for _, state := range doc.States {
		if _, exists := states[state.ID]; exists {
			t.Fatalf("duplicate conformance state id: %s", state.ID)
		}
		states[state.ID] = state.State
	}

	coverage := map[string]map[string]int{}
	for i, vector := range doc.Vectors {
		input, ok := states[vector.State]
		if !ok {
			t.Fatalf("vector %d references unknown state %q", i, vector.State)
		}
		if coverage[vector.Action.Name] == nil {
			coverage[vector.Action.Name] = map[string]int{}
		}
		coverage[vector.Action.Name][vector.Outcome.Kind]++

		name := fmt.Sprintf("%04d_%s_%s", i, vector.Action.Name, vector.Outcome.Kind)
		t.Run(name, func(t *testing.T) {
			seat, logicalNow := seatDocFromFSLState(t, input)
			beforeSeat := seat
			beforeNow := logicalNow

			err := applySeatConformanceAction(&seat, &logicalNow, vector.Action)
			switch vector.Outcome.Kind {
			case "ok":
				if err != nil {
					t.Fatalf("Go implementation rejected FSL-enabled action: %v", err)
				}
			case "requires_failed":
				if err == nil {
					t.Fatal("Go implementation accepted an FSL-disabled action")
				}
				if !reflect.DeepEqual(seat, beforeSeat) || logicalNow != beforeNow {
					t.Fatalf("disabled action mutated state: before=%+v/%d after=%+v/%d", beforeSeat, beforeNow, seat, logicalNow)
				}
			default:
				t.Fatalf("unsupported FSL outcome %q; classify the mismatch instead of skipping the vector", vector.Outcome.Kind)
			}

			got := projectSeatToFSLState(seat, logicalNow)
			if got != vector.Outcome.State {
				t.Fatalf("state mismatch\ninput:    %+v\naction:   %+v\nexpected: %+v\ngot:      %+v", input, vector.Action, vector.Outcome.State, got)
			}
			if changed := got != input; changed != vector.Outcome.StateChanged {
				t.Fatalf("state_changed mismatch: expected=%v got=%v", vector.Outcome.StateChanged, changed)
			}
		})
	}

	for _, action := range []string{
		"start_break",
		"resume_work",
		"set_work_duration",
		"extend_work_duration",
		"extend_break_duration",
	} {
		if coverage[action]["ok"] == 0 {
			t.Errorf("conformance corpus has no successful vector for %s", action)
		}
		if coverage[action]["requires_failed"] == 0 {
			t.Errorf("conformance corpus has no disabled vector for %s", action)
		}
	}
}

func seatDocFromFSLState(t *testing.T, state seatFSLState) (SeatDoc, int) {
	t.Helper()

	var mode SeatState
	switch state.Mode {
	case "Work":
		mode = WorkState
	case "Break":
		mode = BreakState
	default:
		t.Fatalf("unknown FSL mode: %q", state.Mode)
	}

	return SeatDoc{
		SeatID:                  1,
		UserID:                  "formalspec-user",
		SessionID:               "formalspec-session",
		State:                   mode,
		EnteredAt:               seatModelEpoch,
		Until:                   seatModelTime(state.Until),
		CurrentStateStartedAt:   seatModelTime(state.CurrentStateStartedAt),
		CurrentStateUntil:       seatModelTime(state.CurrentStateUntil),
		CurrentSegmentStartedAt: seatModelTime(state.CurrentSegmentStartedAt),
		CumulativeWorkSec:       state.CumulativeWork * 60,
	}, state.Now
}

func applySeatConformanceAction(seat *SeatDoc, logicalNow *int, action seatConformanceAction) error {
	now := seatModelTime(*logicalNow)

	switch action.Name {
	case "tick":
		if *logicalNow >= seatModelClockMax {
			return errors.New("model clock upper bound reached")
		}
		(*logicalNow)++
		return nil
	case "start_break":
		duration, err := seatConformanceParam(action, "duration")
		if err != nil {
			return err
		}
		return seat.StartBreak(now, "formalspec-break", duration)
	case "resume_work":
		return seat.ResumeWork(now, "formalspec-work")
	case "set_work_duration":
		duration, err := seatConformanceParam(action, "duration")
		if err != nil {
			return err
		}
		return seat.SetWorkDuration(now.Add(time.Duration(duration) * time.Minute))
	case "extend_work_duration":
		add, err := seatConformanceParam(action, "add")
		if err != nil {
			return err
		}
		maxRemaining, err := seatConformanceParam(action, "max_remaining")
		if err != nil {
			return err
		}
		_, _, err = seat.ExtendWorkDuration(now, add, maxRemaining)
		return err
	case "extend_break_duration":
		add, err := seatConformanceParam(action, "add")
		if err != nil {
			return err
		}
		maxBreakDuration, err := seatConformanceParam(action, "max_break_duration")
		if err != nil {
			return err
		}
		_, _, _, err = seat.ExtendBreakDuration(now, add, maxBreakDuration)
		return err
	default:
		return fmt.Errorf("unsupported SeatSession action: %s", action.Name)
	}
}

func seatConformanceParam(action seatConformanceAction, name string) (int, error) {
	value, ok := action.Params[name]
	if !ok {
		return 0, fmt.Errorf("action %s is missing parameter %s", action.Name, name)
	}
	return value, nil
}

func projectSeatToFSLState(seat SeatDoc, logicalNow int) seatFSLState {
	mode := string(seat.State)
	switch seat.State {
	case WorkState:
		mode = "Work"
	case BreakState:
		mode = "Break"
	}

	return seatFSLState{
		Mode:                    mode,
		Now:                     logicalNow,
		Until:                   seatModelMinute(seat.Until),
		CurrentStateStartedAt:   seatModelMinute(seat.CurrentStateStartedAt),
		CurrentStateUntil:       seatModelMinute(seat.CurrentStateUntil),
		CurrentSegmentStartedAt: seatModelMinute(seat.CurrentSegmentStartedAt),
		CumulativeWork:          seat.CumulativeWorkSec / 60,
	}
}

func seatModelTime(minute int) time.Time {
	return seatModelEpoch.Add(time.Duration(minute) * time.Minute)
}

func seatModelMinute(value time.Time) int {
	return int(value.Sub(seatModelEpoch) / time.Minute)
}
