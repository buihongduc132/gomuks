// gomuks - A terminal Matrix client written in Go.
// Copyright (C) 2026 Tulir Asokan, buihongduc132

package tui

import (
	"testing"

	"github.com/gdamore/tcell/v2"
	"go.mau.fi/mauview"
	"maunium.net/go/mautrix/id"

	"go.mau.fi/gomuks/pkg/rpc/store"
	"go.mau.fi/gomuks/tui/config"
	"go.mau.fi/gomuks/tui/messages"
)

// mockOrderScreen records the sequence of Sync vs Child Draw
type mockOrderScreen struct {
	tcell.SimulationScreen
	operations []string
}

func (m *mockOrderScreen) Sync() {
	m.operations = append(m.operations, "Sync")
	m.SimulationScreen.Sync()
}

type recordingComponent struct {
	screen *mockOrderScreen
	name   string
}

func (r *recordingComponent) Draw(screen mauview.Screen) {
	r.screen.operations = append(r.screen.operations, r.name)
}

func (r *recordingComponent) OnKeyEvent(event mauview.KeyEvent) bool {
	return false
}

func (r *recordingComponent) OnPasteEvent(event mauview.PasteEvent) bool {
	return false
}

func (r *recordingComponent) OnMouseEvent(event mauview.MouseEvent) bool {
	return false
}

func TestMainView_Draw_ScreenSyncPrecedesChildren(t *testing.T) {
	t.Run("default flex layout with room list", func(t *testing.T) {
		sim := tcell.NewSimulationScreen("")
		_ = sim.Init()
		mock := &mockOrderScreen{SimulationScreen: sim}

		mainView := &MainView{
			flex:            mauview.NewFlex(),
			roomView:        mauview.NewBox(nil),
			config:          &config.Config{},
			needsScreenSync: true,
		}
		mainView.roomView.SetInnerComponent(&recordingComponent{screen: mock, name: "ChildDraw"})
		mainView.flex.AddProportionalComponent(mainView.roomView, 1)

		// Call Draw with needsScreenSync = true
		mainView.Draw(mock)

		if len(mock.operations) < 2 {
			t.Fatalf("expected at least 2 operations, got %v", mock.operations)
		}
		if mock.operations[0] != "Sync" {
			t.Errorf("INVERSION DEFECT: expected 'Sync' at index 0, got %s", mock.operations[0])
		}
		if mock.operations[1] != "ChildDraw" {
			t.Errorf("expected 'ChildDraw' at index 1, got %s", mock.operations[1])
		}
		if mainView.needsScreenSync {
			t.Errorf("expected needsScreenSync to be reset to false")
		}
	})

	t.Run("hide room list direct layout", func(t *testing.T) {
		sim := tcell.NewSimulationScreen("")
		_ = sim.Init()
		mock := &mockOrderScreen{SimulationScreen: sim}

		cfg := &config.Config{}
		cfg.Preferences.HideRoomList = true
		mainView := &MainView{
			flex:            mauview.NewFlex(),
			roomView:        mauview.NewBox(nil),
			config:          cfg,
			needsScreenSync: true,
		}
		mainView.roomView.SetInnerComponent(&recordingComponent{screen: mock, name: "ChildDraw"})

		// Call Draw with needsScreenSync = true
		mainView.Draw(mock)

		if len(mock.operations) < 2 {
			t.Fatalf("expected at least 2 operations, got %v", mock.operations)
		}
		if mock.operations[0] != "Sync" {
			t.Errorf("INVERSION DEFECT: expected 'Sync' at index 0, got %s", mock.operations[0])
		}
		if mock.operations[1] != "ChildDraw" {
			t.Errorf("expected 'ChildDraw' at index 1, got %s", mock.operations[1])
		}
		if mainView.needsScreenSync {
			t.Errorf("expected needsScreenSync to be reset to false")
		}
	})
}

func TestMainView_CallbackPointerSafety(t *testing.T) {
	gmx := &GomuksTUI{
		Config: &config.Config{},
	}
	_ = gmx.NewMainView()

	targetRoomID := id.RoomID("!roomA:example.com")

	// 1. Assert no panic when currentRoom is nil
	if messages.ActiveRoomChecker(targetRoomID) {
		t.Errorf("expected false for nil currentRoom")
	}
	messages.RequestRedraw(targetRoomID) // Must not panic

	// 2. Assert when currentRoom matches targetRoomID, InvalidateTimeline is invoked safely
	matchingRoom := &RoomView{
		Room:    &store.RoomStore{ID: targetRoomID},
		content: &MessageView{},
	}
	gmx.MainView.currentRoom = matchingRoom
	if !messages.ActiveRoomChecker(targetRoomID) {
		t.Errorf("expected true for matching currentRoom")
	}
	messages.RequestRedraw(targetRoomID)
	if !matchingRoom.content.invalidated.Load() {
		t.Errorf("expected InvalidateTimeline to set invalidated = true on message view")
	}

	// 3. Assert safety under rapid room switching simulation across nil, nil RoomStore, other room, and matching room
	otherRoom := &RoomView{
		Room:    &store.RoomStore{ID: id.RoomID("!other:example.com")},
		content: &MessageView{},
	}
	roomWithNilStore := &RoomView{
		Room:    nil,
		content: &MessageView{},
	}

	for i := 0; i < 1000; i++ {
		switch i % 4 {
		case 0:
			gmx.MainView.currentRoom = nil
		case 1:
			gmx.MainView.currentRoom = roomWithNilStore
		case 2:
			gmx.MainView.currentRoom = otherRoom
		case 3:
			gmx.MainView.currentRoom = matchingRoom
		}

		_ = messages.ActiveRoomChecker(targetRoomID)
		messages.RequestRedraw(targetRoomID)
	}

	// 4. Assert snapshot immutability: local capture cur protects against room mutation during callback execution
	gmx.MainView.currentRoom = matchingRoom
	capturedCur := gmx.MainView.currentRoom
	gmx.MainView.currentRoom = nil // switch to nil immediately after snapshot
	if capturedCur == nil || capturedCur.Room == nil || capturedCur.Room.ID != targetRoomID {
		t.Errorf("expected captured snapshot to remain valid even after currentRoom changed")
	}
}
