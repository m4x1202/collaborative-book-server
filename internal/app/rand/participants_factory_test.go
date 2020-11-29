package factory

import (
	"fmt"
	"strconv"
	"testing"
)

func Benchmark_GenerateParticipants(b *testing.B) {
	for playerNum := 1; playerNum <= 10; playerNum++ {
		var players []string
		for i := 0; i < playerNum; i++ {
			players = append(players, strconv.Itoa(i))
		}
		factory := NewParticipantsFactory(players)
		b.Run(fmt.Sprintf("%d players", playerNum), func(b *testing.B) {
			for stages := 1; stages <= 10; stages++ {
				b.Run(fmt.Sprintf("%d stages", stages), func(b *testing.B) {
					failedCount := 0
					for i := 0; i < b.N; i++ {
						b.StartTimer()
						matchmaking, err := factory.Generate(stages)
						b.StopTimer()
						if err != nil {
							b.Log(err)
							failedCount++
							continue
						}
						if len(matchmaking) != playerNum {
							b.FailNow()
						}
						for player, participants := range matchmaking {
							//b.Log(player)
							if participants == nil {
								b.FailNow()
							}
							if len(participants) != stages {
								b.FailNow()
							}
							if participants[1] != player {
								b.FailNow()
							}
							if !participants.ConditionsMet(playerNum) {
								b.FailNow()
							}
						}
					}
					if failedCount > 0 {
						b.Logf("%d failed generations out of %d", failedCount, b.N)
						if float64(failedCount)/float64(b.N) > .2 {
							b.Fatal("More than 20% of the matches failed to generate")
						}
					}
				})
			}
		})
	}
}
