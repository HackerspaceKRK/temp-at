package main

import "testing"

func TestExitBoardComputeCode(t *testing.T) {
	room := RoomConfig{
		ID: "living_room",
		Entities: []EntityConfig{
			{ID: "light1", Representation: exitBoardReprLight},
			{ID: "light2", Representation: exitBoardReprLight},
			{ID: "window1"}, // window matched by vdev type, not representation
			{ID: "temp1", Representation: "temperature"},
		},
	}

	// device holds a virtual device's type + state to seed the manager.
	type device struct {
		typ   VdevType
		state any
	}

	cases := []struct {
		name    string
		devices map[string]device
		want    int
	}{
		{
			name: "all off and closed",
			devices: map[string]device{
				"light1":  {VdevTypeRelay, "OFF"},
				"light2":  {VdevTypeRelay, "OFF"},
				"window1": {VdevTypeContact, true},
			},
			want: 0,
		},
		{
			name:    "unknown states ignored",
			devices: map[string]device{},
			want:    0,
		},
		{
			name: "a light on, windows closed",
			devices: map[string]device{
				"light1":  {VdevTypeRelay, "OFF"},
				"light2":  {VdevTypeRelay, "on"},
				"window1": {VdevTypeContact, true},
			},
			want: 1,
		},
		{
			name: "window open takes priority over lights on",
			devices: map[string]device{
				"light1":  {VdevTypeRelay, "ON"},
				"window1": {VdevTypeContact, false},
			},
			want: 2,
		},
		{
			name: "window open with all lights off",
			devices: map[string]device{
				"light1":  {VdevTypeRelay, "OFF"},
				"window1": {VdevTypeContact, false},
			},
			want: 2,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mgr := NewVdevManager()
			devs := make([]*VirtualDevice, 0, len(tc.devices))
			for id, d := range tc.devices {
				devs = append(devs, &VirtualDevice{ID: id, Type: d.typ, State: d.state})
			}
			mgr.AddDevices(devs)

			s := &ExitBoardService{vdev: mgr}
			got, relevant := s.computeCode(room)
			if !relevant {
				t.Fatalf("expected room to be relevant")
			}
			if got != tc.want {
				t.Fatalf("computeCode = %d, want %d", got, tc.want)
			}
		})
	}
}

func TestExitBoardIrrelevantRoom(t *testing.T) {
	room := RoomConfig{
		ID: "storage",
		Entities: []EntityConfig{
			{ID: "temp1", Representation: "temperature"},
		},
	}
	mgr := NewVdevManager()
	mgr.AddDevices([]*VirtualDevice{{ID: "temp1", Type: VdevTypeTemperature, State: 21.0}})

	s := &ExitBoardService{vdev: mgr}
	if _, relevant := s.computeCode(room); relevant {
		t.Fatalf("expected room with no light/contact entity to be irrelevant")
	}
}
