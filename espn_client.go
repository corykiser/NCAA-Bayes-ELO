package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"
)

const (
	espnBaseURL = "https://site.api.espn.com/apis/site/v2/sports/basketball/mens-college-basketball"
)

// ESPNClient handles requests to ESPN's undocumented API
type ESPNClient struct {
	httpClient *http.Client
}

// NewESPNClient creates a new ESPN API client
func NewESPNClient() *ESPNClient {
	return &ESPNClient{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// ESPNScoreboardResponse represents the top-level scoreboard response
type ESPNScoreboardResponse struct {
	Events []ESPNEvent `json:"events"`
}

// ESPNEvent represents a single game event
type ESPNEvent struct {
	ID           string           `json:"id"`
	Date         string           `json:"date"`
	Name         string           `json:"name"`
	ShortName    string           `json:"shortName"`
	Competitions []ESPNCompetition `json:"competitions"`
	Status       ESPNStatus       `json:"status"`
}

// ESPNCompetition represents the competition details
type ESPNCompetition struct {
	ID              string           `json:"id"`
	Date            string           `json:"date"`
	Attendance      int              `json:"attendance"`
	NeutralSite     bool             `json:"neutralSite"`
	Competitors     []ESPNCompetitor `json:"competitors"`
	Status          ESPNStatus       `json:"status"`
}

// ESPNCompetitor represents a team in the competition
type ESPNCompetitor struct {
	ID       string    `json:"id"`
	HomeAway string    `json:"homeAway"`
	Winner   bool      `json:"winner"`
	Team     ESPNTeam  `json:"team"`
	Score    string    `json:"score"`
}

// ESPNTeam represents team details
type ESPNTeam struct {
	ID               string `json:"id"`
	Name             string `json:"name"`
	Abbreviation     string `json:"abbreviation"`
	DisplayName      string `json:"displayName"`
	ShortDisplayName string `json:"shortDisplayName"`
	Location         string `json:"location"`
}

// ESPNStatus represents the game status
type ESPNStatus struct {
	Clock        float64          `json:"clock"`
	DisplayClock string           `json:"displayClock"`
	Period       int              `json:"period"`
	Type         ESPNStatusType   `json:"type"`
}

// ESPNStatusType represents the status type details
type ESPNStatusType struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	State       string `json:"state"`
	Completed   bool   `json:"completed"`
	Description string `json:"description"`
}

// Game represents a normalized game record for our ELO system
type Game struct {
	Date        time.Time
	HomeTeamID  string
	HomeTeam    string
	AwayTeamID  string
	AwayTeam    string
	HomeScore   int
	AwayScore   int
	NeutralSite bool
	Completed   bool
	WinnerID    string
}

// GetScoreboard fetches games for a specific date (format: YYYYMMDD)
func (c *ESPNClient) GetScoreboard(date string) ([]Game, error) {
	url := fmt.Sprintf("%s/scoreboard?dates=%s&limit=500", espnBaseURL, date)

	resp, err := c.httpClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch scoreboard: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ESPN API returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	var scoreboardResp ESPNScoreboardResponse
	if err := json.Unmarshal(body, &scoreboardResp); err != nil {
		return nil, fmt.Errorf("failed to parse scoreboard response: %w", err)
	}

	return c.parseEvents(scoreboardResp.Events), nil
}

// GetScoreboardRange fetches games for a date range
func (c *ESPNClient) GetScoreboardRange(startDate, endDate time.Time) ([]Game, error) {
	var allGames []Game

	current := startDate
	for !current.After(endDate) {
		dateStr := current.Format("20060102")
		games, err := c.GetScoreboard(dateStr)
		if err != nil {
			// Log error but continue
			fmt.Printf("Warning: failed to fetch games for %s: %v\n", dateStr, err)
		} else {
			allGames = append(allGames, games...)
		}

		current = current.AddDate(0, 0, 1)

		// Rate limiting - be nice to ESPN
		time.Sleep(100 * time.Millisecond)
	}

	return allGames, nil
}

// GetSeason fetches all games for a season (November to April)
func (c *ESPNClient) GetSeason(year int) ([]Game, error) {
	// NCAA basketball season runs roughly November to early April
	// The "year" represents the spring year (e.g., 2025 season = Nov 2024 - Apr 2025)
	startDate := time.Date(year-1, time.November, 1, 0, 0, 0, 0, time.UTC)
	endDate := time.Date(year, time.April, 15, 0, 0, 0, 0, time.UTC)

	// If we're asking for current/future season, end at today
	if endDate.After(time.Now()) {
		endDate = time.Now()
	}

	fmt.Printf("Fetching games from %s to %s...\n", startDate.Format("2006-01-02"), endDate.Format("2006-01-02"))

	return c.GetScoreboardRange(startDate, endDate)
}

// parseEvents converts ESPN events to our Game format
func (c *ESPNClient) parseEvents(events []ESPNEvent) []Game {
	var games []Game

	for _, event := range events {
		if len(event.Competitions) == 0 {
			continue
		}

		comp := event.Competitions[0]
		if len(comp.Competitors) != 2 {
			continue
		}

		var homeTeam, awayTeam *ESPNCompetitor
		for i := range comp.Competitors {
			if comp.Competitors[i].HomeAway == "home" {
				homeTeam = &comp.Competitors[i]
			} else {
				awayTeam = &comp.Competitors[i]
			}
		}

		if homeTeam == nil || awayTeam == nil {
			continue
		}

		gameDate, _ := time.Parse(time.RFC3339, event.Date)
		homeScore, _ := strconv.Atoi(homeTeam.Score)
		awayScore, _ := strconv.Atoi(awayTeam.Score)

		game := Game{
			Date:        gameDate,
			HomeTeamID:  homeTeam.Team.ID,
			HomeTeam:    homeTeam.Team.DisplayName,
			AwayTeamID:  awayTeam.Team.ID,
			AwayTeam:    awayTeam.Team.DisplayName,
			HomeScore:   homeScore,
			AwayScore:   awayScore,
			NeutralSite: comp.NeutralSite,
			Completed:   comp.Status.Type.Completed,
		}

		// Determine winner
		if game.Completed {
			if homeScore > awayScore {
				game.WinnerID = homeTeam.Team.ID
			} else if awayScore > homeScore {
				game.WinnerID = awayTeam.Team.ID
			}
		}

		games = append(games, game)
	}

	return games
}
