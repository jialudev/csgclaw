package im

import "testing"

func TestOnDemandRouting(t *testing.T) {
	room := Room{ID: "room-test", CollaborationMode: "on_demand", ManagerID: "u-manager", Members: []string{"u-admin", "u-manager", "u-dev", "u-qa"}, NotifyAllAgents: true}
	cases := []struct {
		name             string
		message          Message
		manager, dev, qa bool
	}{
		{"unmentioned user", Message{SenderID: "u-admin", Kind: MessageKindMessage}, true, false, false},
		{"multiple mentions", Message{SenderID: "u-admin", Kind: MessageKindMessage, Mentions: []Mention{{ID: "u-dev"}, {ID: "u-qa"}}}, true, false, false},
		{"manager reply", Message{SenderID: "u-manager", Kind: MessageKindMessage, Mentions: []Mention{{ID: "u-dev"}}}, false, false, false},
		{"worker asks QA", Message{SenderID: "u-dev", Kind: MessageKindMessage, Mentions: []Mention{{ID: "u-qa"}}}, true, false, false},
		{"dispatch", Message{SenderID: "u-manager", Kind: MessageKindEvent, Event: &EventPayload{Key: "task_assigned"}, Mentions: []Mention{{ID: "u-dev"}}}, false, true, false},
		{"activity", Message{SenderID: "u-dev", Kind: MessageKindEvent, Event: &EventPayload{Key: "agent_activity"}}, false, false, false},
		{"feedback", Message{SenderID: "u-admin", Kind: MessageKindEvent, Event: &EventPayload{Key: "task_feedback"}, Mentions: []Mention{{ID: "u-manager"}}}, true, false, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			for i, id := range []string{"pt-manager", "pt-dev", "pt-qa"} {
				want := []bool{tc.manager, tc.dev, tc.qa}[i]
				if got := shouldNotifyParticipant(room, tc.message, id); got != want {
					t.Fatalf("%s: got %v want %v", id, got, want)
				}
			}
		})
	}
}
