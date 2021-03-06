package cb

import (
	"testing"
)

func Test_ParticipantsToStringMap(t *testing.T) {
	participants := Participants{
		1: "a",
	}
	result := participants.ToStringMap()
	val, ok := result["1"]
	if !ok {
		t.FailNow()
	}
	if val != "a" {
		t.FailNow()
	}
}

func Test_ParticipantsConditionsMet1Stage(t *testing.T) {
	participants := Participants{
		1: "a",
	}
	condMet := participants.ConditionsMet(2)
	if !condMet {
		t.FailNow()
	}
}

func Test_ParticipantsConditionsMet(t *testing.T) {
	participants := Participants{
		1: "a",
		2: "b",
	}
	condMet := participants.ConditionsMet(2)
	if !condMet {
		t.FailNow()
	}
}

func Test_ParticipantsConditionsMet1Player(t *testing.T) {
	participants := Participants{
		1: "a",
		2: "a",
	}
	condMet := participants.ConditionsMet(1)
	if !condMet {
		t.FailNow()
	}
}

func Test_ParticipantsConditionsNotMet1Player(t *testing.T) {
	participants := Participants{
		1: "a",
		2: "b",
	}
	condMet := participants.ConditionsMet(1)
	if condMet {
		t.FailNow()
	}
}

func Test_ParticipantsConditionsNotMet(t *testing.T) {
	participants := Participants{
		1: "a",
		2: "a",
	}
	condMet := participants.ConditionsMet(2)
	if condMet {
		t.FailNow()
	}
}

func Test_ParticipantsConditionsMet2(t *testing.T) {
	participants := Participants{
		1: "a",
		2: "b",
		3: "a",
	}
	condMet := participants.ConditionsMet(2)
	if !condMet {
		t.FailNow()
	}
}

func Test_ParticipantsConditionsNotMet2(t *testing.T) {
	participants := Participants{
		1: "a",
		2: "b",
		3: "c",
	}
	condMet := participants.ConditionsMet(2)
	if condMet {
		t.FailNow()
	}
}

func Test_ParticipantsConditionsNotMet3(t *testing.T) {
	participants := Participants{
		1: "a",
		2: "b",
		3: "a",
	}
	condMet := participants.ConditionsMet(3)
	if condMet {
		t.FailNow()
	}
}

func Test_ParticipantsWouldConditionsBeMet(t *testing.T) {
	participants := Participants{
		1: "a",
	}
	condMet := participants.ParticipantWouldMeetConditions("b", 2)
	if !condMet {
		t.FailNow()
	}
}

func Test_ParticipantsWouldConditionsNotBeMet(t *testing.T) {
	participants := Participants{
		1: "a",
	}
	condMet := participants.ParticipantWouldMeetConditions("a", 2)
	if condMet {
		t.FailNow()
	}
}

func Test_ParticipantsWouldConditionsBeMet2(t *testing.T) {
	participants := Participants{
		1: "a",
		2: "b",
	}
	condMet := participants.ParticipantWouldMeetConditions("a", 2)
	if !condMet {
		t.FailNow()
	}
}

func Test_ParticipantsWouldConditionsNotBeMet2(t *testing.T) {
	participants := Participants{
		1: "a",
		2: "b",
	}
	condMet := participants.ParticipantWouldMeetConditions("a", 3)
	if condMet {
		t.FailNow()
	}
}
