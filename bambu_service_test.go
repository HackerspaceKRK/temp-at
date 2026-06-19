package main

import (
	"encoding/json"
	"testing"
)

// sampleReport is a trimmed real message captured from the printer
// (device/<serial>/report) while a print named "e-ink" was running at 69%.
const sampleReport = `{
  "print": {
    "command": "push_status",
    "gcode_state": "RUNNING",
    "gcode_file": "/data/Metadata/plate_1.gcode",
    "subtask_name": "e-ink",
    "mc_percent": 69,
    "mc_remaining_time": 13,
    "mc_print_error_code": "0",
    "print_error": 0,
    "gcode_start_time": "1781901787",
    "task_id": "5137",
    "layer_num": 9,
    "total_layer_num": 60,
    "nozzle_temper": 220.0,
    "nozzle_target_temper": 220.0,
    "bed_temper": 55.0,
    "bed_target_temper": 55.0,
    "chamber_temper": 30.0
  }
}`

func parseSample(t *testing.T) map[string]any {
	t.Helper()
	var msg struct {
		Print map[string]any `json:"print"`
	}
	if err := json.Unmarshal([]byte(sampleReport), &msg); err != nil {
		t.Fatalf("unmarshal sample: %v", err)
	}
	return msg.Print
}

func TestDeriveBambuState_Running(t *testing.T) {
	st := deriveBambuState(parseSample(t))

	if st.State != "printing" {
		t.Errorf("State = %q, want printing", st.State)
	}
	if st.Progress != 69 {
		t.Errorf("Progress = %d, want 69", st.Progress)
	}
	if st.RemainingTime != 13 {
		t.Errorf("RemainingTime = %d, want 13", st.RemainingTime)
	}
	if st.Filename != "e-ink" {
		t.Errorf("Filename = %q, want e-ink", st.Filename)
	}
	if st.TaskID != "5137" {
		t.Errorf("TaskID = %q, want 5137", st.TaskID)
	}
	if st.LayerNum != 9 || st.TotalLayerNum != 60 {
		t.Errorf("layers = %d/%d, want 9/60", st.LayerNum, st.TotalLayerNum)
	}
	if st.NozzleTemp != 220 || st.BedTemp != 55 || st.ChamberTemp != 30 {
		t.Errorf("temps nozzle=%v bed=%v chamber=%v", st.NozzleTemp, st.BedTemp, st.ChamberTemp)
	}
	if st.StartedAt != 1781901787*1000 {
		t.Errorf("StartedAt = %d, want %d (gcode_start_time seconds -> millis)", st.StartedAt, int64(1781901787)*1000)
	}
}

func TestDeriveBambuState_FilenameFallback(t *testing.T) {
	full := map[string]any{
		"gcode_state": "RUNNING",
		"gcode_file":  "/data/Metadata/plate_2.gcode",
	}
	st := deriveBambuState(full)
	if st.Filename != "plate_2" {
		t.Errorf("Filename = %q, want plate_2 (basename without ext)", st.Filename)
	}
}

func TestDeriveBambuState_StateMapping(t *testing.T) {
	cases := map[string]string{
		"IDLE":    "idle",
		"PREPARE": "printing",
		"PAUSE":   "paused",
		"FINISH":  "finished",
		"FAILED":  "failed",
	}
	for gcode, want := range cases {
		st := deriveBambuState(map[string]any{"gcode_state": gcode})
		if st.State != want {
			t.Errorf("gcode_state %q -> %q, want %q", gcode, st.State, want)
		}
	}
}

func TestDeriveBambuState_PrintErrorFails(t *testing.T) {
	st := deriveBambuState(map[string]any{
		"gcode_state": "RUNNING",
		"print_error": float64(83886109),
	})
	if st.State != "failed" {
		t.Errorf("State = %q, want failed (non-zero print_error)", st.State)
	}
}
