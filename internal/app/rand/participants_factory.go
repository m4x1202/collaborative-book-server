package factory

import (
	"fmt"
	"math/rand"
	"time"

	cb "github.com/m4x1202/collaborative-book"
)

const (
	maxAttempts  = 5
	maxPermRetry = 50
)

//Ensure ParticipantsFactory implements cb.ParticipantsFactory
var _ cb.ParticipantsFactory = (*ParticipantsFactory)(nil)

type ParticipantsFactory struct {
	AvailablePlayers []string
	NumPlayers       int
}

func NewParticipantsFactory(players []string) *ParticipantsFactory {
	return &ParticipantsFactory{
		AvailablePlayers: players,
		NumPlayers:       len(players),
	}
}

func (pf *ParticipantsFactory) Generate(numStages int) (map[string]cb.Participants, error) {
	result := make(map[string]cb.Participants, pf.NumPlayers)

	for _, player := range pf.AvailablePlayers {
		result[player] = make(cb.Participants, numStages)
		result[player][1] = player
	}
	if numStages == 1 {
		return result, nil
	}

	for stage := 2; stage <= numStages; stage++ {
		if pf.NumPlayers == 1 {
			result[pf.AvailablePlayers[0]][stage] = pf.AvailablePlayers[0]
			continue
		}

		var matchingForStage map[string]string
		var err error
		for i := 0; i < maxAttempts; i++ {
			rand.Seed(time.Now().UnixNano())

			matchingForStage, err = generateMatchingsForStage(result, pf.AvailablePlayers, pf.NumPlayers)
			if err == nil {
				break
			}
		}
		if err != nil {
			return nil, err
		}
		for name, participants := range result {
			participants[stage] = matchingForStage[name]
			if !participants.ConditionsMet(pf.NumPlayers) {
				return nil, fmt.Errorf("Participant conditions not met")
			}
		}
	}

	return result, nil
}

func generateMatchingsForStage(matchingSoFar map[string]cb.Participants, availPlayers []string, numPlayer int) (map[string]string, error) {
	available := make([]string, numPlayer)
	var matchingForStage map[string]string
	iterations := 0
	conditionsBroken := true
	for conditionsBroken {
		if iterations >= maxPermRetry {
			return nil, fmt.Errorf("No valid matching found after %d iterations", iterations)
		}
		iterations++
		matchingForStage = make(map[string]string, numPlayer)
		conditionsBroken = false
		perm := rand.Perm(numPlayer)
		for i, v := range perm {
			available[v] = availPlayers[i]
		}
		count := 0
		for player, participants := range matchingSoFar {
			if !participants.ParticipantWouldMeetConditions(available[count], numPlayer) {
				conditionsBroken = true
				break
			}
			matchingForStage[player] = available[count]
			count++
		}
	}
	return matchingForStage, nil
}
