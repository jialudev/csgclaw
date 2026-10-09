package im

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func BenchmarkSessionPersistence(b *testing.B) {
	for _, tc := range []struct {
		name                                  string
		rooms, messages, bodyBytes, blobEvery int
	}{
		{"one_room_1000_messages", 1, 1000, 1024, 0},
		{"twenty_rooms_100_messages", 20, 100, 256, 0},
		{"ten_rooms_50_large_bodies", 10, 50, 256, 10},
	} {
		b.Run(tc.name, func(b *testing.B) {
			path := filepath.Join(b.TempDir(), "im", "state.json")
			state := Bootstrap{CurrentUserID: AdminUserID, Users: []User{{ID: AdminUserID, Name: "Admin"}}}
			now := time.Date(2026, 10, 9, 0, 0, 0, 0, time.UTC)
			for r := 0; r < tc.rooms; r++ {
				room := Room{ID: fmt.Sprintf("room-%d", r), Title: "Room", Members: []string{AdminUserID}}
				for m := 0; m < tc.messages; m++ {
					size := tc.bodyBytes
					if tc.blobEvery > 0 && m%tc.blobEvery == 0 {
						size = 70 * 1024
					}
					room.Messages = append(room.Messages, Message{ID: fmt.Sprintf("msg-%d", m), SenderID: AdminUserID, Content: strings.Repeat("A", size), CreatedAt: now})
				}
				state.Rooms = append(state.Rooms, room)
			}
			if err := SaveBootstrap(path, state); err != nil {
				b.Fatal(err)
			}
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				state.Rooms[0].Messages[tc.messages-1].Content = fmt.Sprintf("new reply %d", i)
				if err := SaveBootstrap(path, state); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// Restore the same history outside the timed section, so append measurements
// compare a fixed workload rather than an ever-growing conversation.
func BenchmarkSessionAppend(b *testing.B) {
	for _, count := range []int{1000, 5000} {
		b.Run(fmt.Sprintf("one_room_%d_messages", count), func(b *testing.B) {
			dir := b.TempDir()
			path := filepath.Join(dir, "state.json")
			now := time.Date(2026, 10, 15, 0, 0, 0, 0, time.UTC)
			room := Room{ID: "room", Title: "Room", Members: []string{AdminUserID}}
			for i := 0; i < count; i++ {
				room.Messages = append(room.Messages, Message{ID: fmt.Sprintf("msg-%d", i), SenderID: AdminUserID, Content: strings.Repeat("A", 1024), CreatedAt: now})
			}
			state := Bootstrap{CurrentUserID: AdminUserID, Users: []User{{ID: AdminUserID, Name: "Admin"}}, Rooms: []Room{room}}
			if err := SaveBootstrap(path, state); err != nil {
				b.Fatal(err)
			}
			sessionPath := filepath.Join(dir, "sessions", "room.jsonl")
			original, err := os.ReadFile(sessionPath)
			if err != nil {
				b.Fatal(err)
			}
			state.Rooms[0].Messages = append(state.Rooms[0].Messages, Message{ID: "new", SenderID: AdminUserID, Content: "New reply", CreatedAt: now})
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				b.StopTimer()
				if err := os.WriteFile(sessionPath, original, 0o600); err != nil {
					b.Fatal(err)
				}
				b.StartTimer()
				if err := SaveBootstrap(path, state); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
