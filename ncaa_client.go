package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"sync"
	"time"
)

const (
	ncaaMaxConcurrentRequests = 5 // NCAA API limits to 5 req/sec
)

const (
	// Public instance of henrygd's NCAA API wrapper
	ncaaAPIBaseURL = "https://ncaa-api.henrygd.me"
)

// NCAAClient handles requests to the NCAA API wrapper
type NCAAClient struct {
	httpClient *http.Client
}

// NewNCAAClient creates a new NCAA API client
func NewNCAAClient() *NCAAClient {
	return &NCAAClient{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// NCAAScoreboardResponse represents the scoreboard response
type NCAAScoreboardResponse struct {
	Games []NCAAGame `json:"games"`
}

// NCAAGame represents a game from the NCAA API
type NCAAGame struct {
	Game NCAAGameDetails `json:"game"`
}

// NCAAGameDetails contains the game details
type NCAAGameDetails struct {
	GameID          string       `json:"gameID"`
	StartDate       string       `json:"startDate"`
	StartTime       string       `json:"startTime"`
	StartTimeEpoch  int64        `json:"startTimeEpoch"`
	GameState       string       `json:"gameState"`
	Home            NCAATeamInfo `json:"home"`
	Away            NCAATeamInfo `json:"away"`
	FinalMessage    string       `json:"finalMessage"`
	CurrentPeriod   string       `json:"currentPeriod"`
	ContestClock    string       `json:"contestClock"`
}

// NCAATeamInfo represents team info in a game
type NCAATeamInfo struct {
	Names      NCAATeamNames `json:"names"`
	Score      string        `json:"score"`
	Winner     bool          `json:"winner"`
	TeamID     string        `json:"teamId"`
}

// NCAATeamNames contains various team name formats
type NCAATeamNames struct {
	Char6    string `json:"char6"`
	Short    string `json:"short"`
	Seo      string `json:"seo"`
	Full     string `json:"full"`
}

// GetScoreboard fetches games for a specific date
// Date format: YYYY/MM/DD
func (c *NCAAClient) GetScoreboard(year, month, day int) ([]Game, error) {
	url := fmt.Sprintf("%s/scoreboard/basketball-men/d1/%d/%02d/%02d", ncaaAPIBaseURL, year, month, day)

	resp, err := c.httpClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch NCAA scoreboard: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("NCAA API returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	var scoreboardResp NCAAScoreboardResponse
	if err := json.Unmarshal(body, &scoreboardResp); err != nil {
		return nil, fmt.Errorf("failed to parse NCAA scoreboard response: %w", err)
	}

	return c.parseGames(scoreboardResp.Games, year, month, day), nil
}

// ncaaDateResult holds the result of fetching a single date
type ncaaDateResult struct {
	date  time.Time
	games []Game
	err   error
}

// GetScoreboardRange fetches games for a date range using parallel requests
func (c *NCAAClient) GetScoreboardRange(startDate, endDate time.Time) ([]Game, error) {
	// Build list of dates to fetch
	var dates []time.Time
	current := startDate
	for !current.After(endDate) {
		dates = append(dates, current)
		current = current.AddDate(0, 0, 1)
	}

	fmt.Printf("Fetching %d days of games using %d parallel workers...\n", len(dates), ncaaMaxConcurrentRequests)

	// Channel for dates to process
	dateChan := make(chan time.Time, len(dates))
	for _, d := range dates {
		dateChan <- d
	}
	close(dateChan)

	// Channel for results
	resultChan := make(chan ncaaDateResult, len(dates))

	// Start worker goroutines
	var wg sync.WaitGroup
	for i := 0; i < ncaaMaxConcurrentRequests; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for date := range dateChan {
				games, err := c.GetScoreboard(date.Year(), int(date.Month()), date.Day())
				resultChan <- ncaaDateResult{date: date, games: games, err: err}
				// Rate limiting - NCAA API limits to 5 req/sec
				time.Sleep(200 * time.Millisecond)
			}
		}()
	}

	// Close result channel when all workers done
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	// Collect results into a map by date
	gamesByDate := make(map[time.Time][]Game)
	errorCount := 0
	for result := range resultChan {
		if result.err != nil {
			errorCount++
		} else {
			gamesByDate[result.date] = result.games
		}
	}

	if errorCount > 0 {
		fmt.Printf("Warning: %d dates had fetch errors\n", errorCount)
	}

	// Combine games in chronological order
	var allGames []Game
	for _, date := range dates {
		if games, ok := gamesByDate[date]; ok {
			allGames = append(allGames, games...)
		}
	}

	return allGames, nil
}

// GetSeason fetches all games for a season
func (c *NCAAClient) GetSeason(year int) ([]Game, error) {
	startDate := time.Date(year-1, time.November, 1, 0, 0, 0, 0, time.UTC)
	endDate := time.Date(year, time.April, 15, 0, 0, 0, 0, time.UTC)

	if endDate.After(time.Now()) {
		endDate = time.Now()
	}

	fmt.Printf("Fetching NCAA games from %s to %s...\n", startDate.Format("2006-01-02"), endDate.Format("2006-01-02"))

	return c.GetScoreboardRange(startDate, endDate)
}

// parseGames converts NCAA games to our Game format
func (c *NCAAClient) parseGames(ncaaGames []NCAAGame, year, month, day int) []Game {
	var games []Game

	for _, ng := range ncaaGames {
		g := ng.Game

		homeScore, _ := strconv.Atoi(g.Home.Score)
		awayScore, _ := strconv.Atoi(g.Away.Score)

		game := Game{
			Date:        time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC),
			HomeTeamID:  g.Home.TeamID,
			HomeTeam:    g.Home.Names.Full,
			AwayTeamID:  g.Away.TeamID,
			AwayTeam:    g.Away.Names.Full,
			HomeScore:   homeScore,
			AwayScore:   awayScore,
			NeutralSite: false, // NCAA API doesn't clearly indicate this
			Completed:   g.GameState == "final",
		}

		if game.Completed {
			if g.Home.Winner {
				game.WinnerID = g.Home.TeamID
			} else if g.Away.Winner {
				game.WinnerID = g.Away.TeamID
			}
		}

		games = append(games, game)
	}

	return games
}
