package onvifsvc

import "testing"

func TestIsRecordingEventMotionActive(t *testing.T) {
	event := Event{Topic: "tns1:RuleEngine/CellMotionDetector/Motion", Data: map[string]string{"IsMotion": "true"}}
	if !IsRecordingEvent(event) {
		t.Fatal("active motion event should trigger recording")
	}
}

func TestIsRecordingEventMotionInactive(t *testing.T) {
	event := Event{Topic: "tns1:RuleEngine/CellMotionDetector/Motion", Data: map[string]string{"IsMotion": "false"}}
	if IsRecordingEvent(event) {
		t.Fatal("inactive motion event should not trigger recording")
	}
}

func TestIsRecordingEventIgnoresUnrelatedEvent(t *testing.T) {
	event := Event{Topic: "tns1:Device/Trigger/Relay", Data: map[string]string{"State": "true"}}
	if IsRecordingEvent(event) {
		t.Fatal("unrelated event should not trigger recording")
	}
}
