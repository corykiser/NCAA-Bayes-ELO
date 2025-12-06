package main

import (
	"fmt"
	"math"
	"sort"
)

// Tuned parameters from cross-validation
const (
	OptimalKFactor    = 0.90   // Tuned K factor for likelihood function
	ELOMin            = 0.0    // Minimum ELO value
	ELOMax            = 3000.0 // Maximum ELO value
	ELOStep           = 5.0    // Step size for discretization
	InitialMean       = 1500.0 // Initial ELO mean
)

// Distribution represents a discrete probability distribution over ELO values
type Distribution struct {
	Values []float64 // ELO values (quantiles)
	Probs  []float64 // Probabilities
}

// NewUniformDistribution creates a uniform prior distribution
func NewUniformDistribution() *Distribution {
	n := int((ELOMax - ELOMin) / ELOStep)
	d := &Distribution{
		Values: make([]float64, n),
		Probs:  make([]float64, n),
	}

	prob := 1.0 / float64(n)
	for i := 0; i < n; i++ {
		d.Values[i] = ELOMin + float64(i)*ELOStep
		d.Probs[i] = prob
	}

	return d
}

// Mean returns the expected value of the distribution
func (d *Distribution) Mean() float64 {
	var sum float64
	for i, v := range d.Values {
		sum += v * d.Probs[i]
	}
	return sum
}

// Std returns the standard deviation of the distribution
func (d *Distribution) Std() float64 {
	mean := d.Mean()
	var variance float64
	for i, v := range d.Values {
		diff := v - mean
		variance += diff * diff * d.Probs[i]
	}
	return math.Sqrt(variance)
}

// Percentile returns the value at the given percentile (0-100)
func (d *Distribution) Percentile(p float64) float64 {
	target := p / 100.0
	var cumulative float64
	for i, prob := range d.Probs {
		cumulative += prob
		if cumulative >= target {
			return d.Values[i]
		}
	}
	return d.Values[len(d.Values)-1]
}

// Normalize ensures probabilities sum to 1
func (d *Distribution) Normalize() {
	var sum float64
	for _, p := range d.Probs {
		sum += p
	}
	if sum > 0 {
		for i := range d.Probs {
			d.Probs[i] /= sum
		}
	}
}

// Clone creates a deep copy of the distribution
func (d *Distribution) Clone() *Distribution {
	clone := &Distribution{
		Values: make([]float64, len(d.Values)),
		Probs:  make([]float64, len(d.Probs)),
	}
	copy(clone.Values, d.Values)
	copy(clone.Probs, d.Probs)
	return clone
}

// TeamRating holds a team's ELO distribution
type TeamRating struct {
	TeamID   string
	TeamName string
	Dist     *Distribution
}

// BayesianELO implements the Bayesian ELO rating system
type BayesianELO struct {
	Teams    map[string]*TeamRating
	KFactor  float64
	GameLog  []GameResult
}

// GameResult stores the result of processing a game
type GameResult struct {
	Date          string
	WinnerName    string
	WinnerID      string
	LoserName     string
	LoserID       string
	WinnerELO     float64
	LoserELO      float64
	WinProb       float64
	HomeAdvantage string // "H", "A", or "N"
}

// NewBayesianELO creates a new Bayesian ELO system
func NewBayesianELO() *BayesianELO {
	return &BayesianELO{
		Teams:   make(map[string]*TeamRating),
		KFactor: OptimalKFactor,
		GameLog: []GameResult{},
	}
}

// getOrCreateTeam gets an existing team or creates a new one with uniform prior
func (b *BayesianELO) getOrCreateTeam(teamID, teamName string) *TeamRating {
	if team, exists := b.Teams[teamID]; exists {
		return team
	}

	team := &TeamRating{
		TeamID:   teamID,
		TeamName: teamName,
		Dist:     NewUniformDistribution(),
	}
	b.Teams[teamID] = team
	return team
}

// winProbability calculates P(team1 wins) given ELO difference
func (b *BayesianELO) winProbability(diff float64) float64 {
	return 1.0 / (1.0 + math.Pow(10, -diff*b.KFactor/400.0))
}

// ProcessGame updates team distributions based on a game result
func (b *BayesianELO) ProcessGame(game Game) {
	if !game.Completed || game.WinnerID == "" {
		return
	}

	var winnerID, winnerName, loserID, loserName string
	var homeAdv string

	if game.HomeScore > game.AwayScore {
		winnerID = game.HomeTeamID
		winnerName = game.HomeTeam
		loserID = game.AwayTeamID
		loserName = game.AwayTeam
		if game.NeutralSite {
			homeAdv = "N"
		} else {
			homeAdv = "H" // Winner was home
		}
	} else {
		winnerID = game.AwayTeamID
		winnerName = game.AwayTeam
		loserID = game.HomeTeamID
		loserName = game.HomeTeam
		if game.NeutralSite {
			homeAdv = "N"
		} else {
			homeAdv = "A" // Winner was away
		}
	}

	winner := b.getOrCreateTeam(winnerID, winnerName)
	loser := b.getOrCreateTeam(loserID, loserName)

	// Record pre-game state
	winnerPreMean := winner.Dist.Mean()
	loserPreMean := loser.Dist.Mean()
	preWinProb := b.winProbability(winnerPreMean - loserPreMean)

	// Compute joint distribution and likelihood
	n := len(winner.Dist.Values)
	jointProbs := make([][]float64, n)
	for i := range jointProbs {
		jointProbs[i] = make([]float64, n)
	}

	// Build joint distribution (outer product of marginals)
	for i := 0; i < n; i++ {
		for j := 0; j < n; j++ {
			jointProbs[i][j] = winner.Dist.Probs[i] * loser.Dist.Probs[j]
		}
	}

	// Apply likelihood (winner won)
	for i := 0; i < n; i++ {
		for j := 0; j < n; j++ {
			diff := winner.Dist.Values[i] - loser.Dist.Values[j]
			likelihood := b.winProbability(diff)
			jointProbs[i][j] *= likelihood
		}
	}

	// Normalize joint
	var totalProb float64
	for i := 0; i < n; i++ {
		for j := 0; j < n; j++ {
			totalProb += jointProbs[i][j]
		}
	}
	if totalProb > 0 {
		for i := 0; i < n; i++ {
			for j := 0; j < n; j++ {
				jointProbs[i][j] /= totalProb
			}
		}
	}

	// Marginalize to get updated distributions
	newWinnerProbs := make([]float64, n)
	newLoserProbs := make([]float64, n)

	for i := 0; i < n; i++ {
		for j := 0; j < n; j++ {
			newWinnerProbs[i] += jointProbs[i][j]
			newLoserProbs[j] += jointProbs[i][j]
		}
	}

	winner.Dist.Probs = newWinnerProbs
	winner.Dist.Normalize()

	loser.Dist.Probs = newLoserProbs
	loser.Dist.Normalize()

	// Log the game result
	b.GameLog = append(b.GameLog, GameResult{
		Date:          game.Date.Format("2006-01-02"),
		WinnerName:    winnerName,
		WinnerID:      winnerID,
		LoserName:     loserName,
		LoserID:       loserID,
		WinnerELO:     winnerPreMean,
		LoserELO:      loserPreMean,
		WinProb:       preWinProb,
		HomeAdvantage: homeAdv,
	})
}

// ProcessGames processes multiple games in order
func (b *BayesianELO) ProcessGames(games []Game) {
	// Sort games by date
	sort.Slice(games, func(i, j int) bool {
		return games[i].Date.Before(games[j].Date)
	})

	for _, game := range games {
		b.ProcessGame(game)
	}
}

// GetRankings returns teams sorted by mean ELO
func (b *BayesianELO) GetRankings() []*TeamRating {
	var rankings []*TeamRating
	for _, team := range b.Teams {
		rankings = append(rankings, team)
	}

	sort.Slice(rankings, func(i, j int) bool {
		return rankings[i].Dist.Mean() > rankings[j].Dist.Mean()
	})

	return rankings
}

// PredictMatchup predicts the probability of team1 beating team2
func (b *BayesianELO) PredictMatchup(team1ID, team2ID string) (float64, error) {
	team1, exists1 := b.Teams[team1ID]
	team2, exists2 := b.Teams[team2ID]

	if !exists1 {
		return 0, fmt.Errorf("team %s not found", team1ID)
	}
	if !exists2 {
		return 0, fmt.Errorf("team %s not found", team2ID)
	}

	// Compute win probability by integrating over joint distribution
	var winProb float64
	for i, p1 := range team1.Dist.Probs {
		for j, p2 := range team2.Dist.Probs {
			diff := team1.Dist.Values[i] - team2.Dist.Values[j]
			prob := b.winProbability(diff)
			winProb += p1 * p2 * prob
		}
	}

	return winProb, nil
}

// PrintTeamDistribution prints a summary of a team's distribution
func (b *BayesianELO) PrintTeamDistribution(teamID string) {
	team, exists := b.Teams[teamID]
	if !exists {
		fmt.Printf("Team %s not found\n", teamID)
		return
	}

	fmt.Printf("\n%s (ID: %s)\n", team.TeamName, team.TeamID)
	fmt.Printf("  Mean ELO: %.1f\n", team.Dist.Mean())
	fmt.Printf("  Std Dev:  %.1f\n", team.Dist.Std())
	fmt.Printf("  5th %%:    %.1f\n", team.Dist.Percentile(5))
	fmt.Printf("  25th %%:   %.1f\n", team.Dist.Percentile(25))
	fmt.Printf("  Median:   %.1f\n", team.Dist.Percentile(50))
	fmt.Printf("  75th %%:   %.1f\n", team.Dist.Percentile(75))
	fmt.Printf("  95th %%:   %.1f\n", team.Dist.Percentile(95))
}
