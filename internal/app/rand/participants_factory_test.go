package factory

import (
	"fmt"
	"math"
	"math/rand"
	"strconv"
	"testing"
	"time"
)

func Test_GenerateParticipants(t *testing.T) {
	for playerNum := 1; playerNum <= 5; playerNum++ {
		var players []string
		for i := 0; i < playerNum; i++ {
			players = append(players, strconv.Itoa(i))
		}
		factory := NewParticipantsFactory(players)
		t.Run(fmt.Sprintf("%d players", playerNum), func(t *testing.T) {
			for stages := 1; stages <= 5; stages++ {
				t.Run(fmt.Sprintf("%d stages", stages), func(t *testing.T) {
					matchmaking, err := factory.Generate(stages)
					if err != nil {
						t.Fatal(err)
					}
					if len(matchmaking) != playerNum {
						t.FailNow()
					}
					for player, participants := range matchmaking {
						if participants == nil {
							t.FailNow()
						}
						if len(participants) != stages {
							t.FailNow()
						}
						if participants[1] != player {
							t.FailNow()
						}
						if !participants.ConditionsMet(playerNum) {
							t.FailNow()
						}
					}
				})
			}
		})
	}
}

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

// Benchmark to see which random method is faster
func Benchmark_RandomMethod(b *testing.B) {
	for k := 1.; k < 8; k++ {
		size := int(math.Pow(2, k))
		test := make([]string, size)
		for j := 0; j < size; j++ {
			test[j] = strconv.Itoa(j)
		}
		rand.Seed(time.Now().UnixNano())
		b.Run(fmt.Sprintf("%d", size), func(b *testing.B) {
			b.Run("perm", func(b *testing.B) {
				testCopy := make([]string, len(test))
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					perm := rand.Perm(size)
					for i, v := range perm {
						testCopy[v] = test[i]
					}
				}
			})
			b.Run("shuffle", func(b *testing.B) {
				testCopy := make([]string, len(test))
				copy(testCopy, test)
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					rand.Shuffle(size, func(i, j int) { testCopy[i], testCopy[j] = testCopy[j], testCopy[i] })
				}
			})
		})
	}
}
