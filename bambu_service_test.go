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

// sampleDetailReport exercises the detail fields (AMS, fans, wifi, lights, HMS)
// with the shapes from a real X1C push_status message.
const sampleDetailReport = `{
  "print": {
    "gcode_state": "IDLE",
    "ams": {
      "ams": [
        {
          "humidity": "4",
          "id": "0",
          "temp": "29.2",
          "tray": [
            {"id": "0", "tray_type": "ABS", "tray_color": "FFAA00FF", "remain": 75, "tray_sub_brands": "ABS-GF"},
            {"id": "1"},
            {"id": "2"},
            {"id": "3"}
          ]
        }
      ]
    },
    "vt_tray": {"id": "254", "tray_type": "", "tray_color": "00000000", "remain": 0},
    "cooling_fan_speed": "15",
    "big_fan1_speed": "7",
    "big_fan2_speed": "0",
    "wifi_signal": "-74dBm",
    "spd_lvl": 2,
    "nozzle_diameter": "0.4",
    "nozzle_type": "hardened_steel",
    "lights_report": [
      {"mode": "on", "node": "chamber_light"},
      {"mode": "flashing", "node": "work_light"}
    ],
    "hms": [{"attr": 50336256, "code": 131073}]
  }
}`

func TestDeriveBambuState_Details(t *testing.T) {
	var msg struct {
		Print map[string]any `json:"print"`
	}
	if err := json.Unmarshal([]byte(sampleDetailReport), &msg); err != nil {
		t.Fatalf("unmarshal sample: %v", err)
	}
	st := deriveBambuState(msg.Print)

	if len(st.Ams) != 1 {
		t.Fatalf("len(Ams) = %d, want 1", len(st.Ams))
	}
	unit := st.Ams[0]
	if unit.Humidity != 4 || unit.Temp != 29.2 {
		t.Errorf("AMS humidity=%d temp=%v, want 4/29.2", unit.Humidity, unit.Temp)
	}
	if len(unit.Trays) != 4 {
		t.Fatalf("len(Trays) = %d, want 4", len(unit.Trays))
	}
	tr := unit.Trays[0]
	if tr.Type != "ABS" || tr.Color != "FFAA00FF" || tr.Remain != 75 || tr.SubBrand != "ABS-GF" || tr.Empty {
		t.Errorf("tray0 = %+v, want ABS/FFAA00FF/75/ABS-GF non-empty", tr)
	}
	if !unit.Trays[1].Empty || unit.Trays[1].Remain != -1 {
		t.Errorf("tray1 = %+v, want empty with remain -1", unit.Trays[1])
	}
	if st.VtTray == nil || !st.VtTray.Empty || st.VtTray.Color != "" {
		t.Errorf("VtTray = %+v, want empty tray with no color", st.VtTray)
	}
	if st.FanCooling != 100 || st.FanAux != 47 || st.FanChamber != 0 {
		t.Errorf("fans = %d/%d/%d, want 100/47/0", st.FanCooling, st.FanAux, st.FanChamber)
	}
	if st.WifiSignal != "-74dBm" || st.SpeedLevel != 2 {
		t.Errorf("wifi=%q spd=%d, want -74dBm/2", st.WifiSignal, st.SpeedLevel)
	}
	if st.NozzleDiameter != "0.4" || st.NozzleType != "hardened_steel" {
		t.Errorf("nozzle = %q %q", st.NozzleDiameter, st.NozzleType)
	}
	if len(st.Lights) != 2 || st.Lights[0].Node != "chamber_light" || st.Lights[1].Mode != "flashing" {
		t.Errorf("Lights = %+v", st.Lights)
	}
	if len(st.Hms) != 1 || st.Hms[0].Attr != 50336256 || st.Hms[0].Code != 131073 {
		t.Errorf("Hms = %+v", st.Hms)
	}
}

func TestShouldAssignState_NonComparableDeepEqual(t *testing.T) {
	a := BambuPrinterState{State: "idle", Ams: []PrinterAmsUnit{{ID: 0, Humidity: 4}}}
	b := BambuPrinterState{State: "idle", Ams: []PrinterAmsUnit{{ID: 0, Humidity: 4}}}
	if shouldAssignState(a, b) {
		t.Error("identical states with slices should not be treated as changed")
	}
	b.Ams[0].Humidity = 5
	if !shouldAssignState(a, b) {
		t.Error("differing states should be treated as changed")
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
