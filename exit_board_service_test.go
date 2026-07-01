package main

import "testing"

func TestExitBoardComputeCode(t *testing.T) {
	room := RoomConfig{
		ID: "living_room",
		Entities: []EntityConfig{
			{ID: "light1", Representation: exitBoardReprLight},
			{ID: "light2", Representation: exitBoardReprLight},
			{ID: "window1", Representation: exitBoardReprWindow},
			{ID: "temp1", Representation: "temperature"}, // ignored
		},
	}

	cases := []struct {
		name   string
		states map[string]any
		want   int
	}{
		{
			name:   "all off and closed",
			states: map[string]any{"light1": "OFF", "light2": "OFF", "window1": true},
			want:   0,
		},
		{
			name:   "unknown states ignored",
			states: map[string]any{},
			want:   0,
		},
		{
			name:   "a light on, windows closed",
			states: map[string]any{"light1": "OFF", "light2": "on", "window1": true},
			want:   1,
		},
		{
			name:   "window open takes priority over lights on",
			states: map[string]any{"light1": "ON", "window1": false},
			want:   2,
		},
		{
			name:   "window open with all lights off",
			states: map[string]any{"light1": "OFF", "window1": false},
			want:   2,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mgr := NewVdevManager()
			devs := make([]*VirtualDevice, 0, len(tc.states))
			for id, st := range tc.states {
				devs = append(devs, &VirtualDevice{ID: id, State: st})
			}
			mgr.AddDevices(devs)

			s := &ExitBoardService{vdev: mgr}
			if got := s.computeCode(room); got != tc.want {
				t.Fatalf("computeCode = %d, want %d", got, tc.want)
			}
		})
	}
}
