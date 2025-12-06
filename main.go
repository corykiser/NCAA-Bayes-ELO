package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"
)

// OutputFormat specifies the output format type
type OutputFormat string

const (
	FormatTable OutputFormat = "table"
	FormatJSON  OutputFormat = "json"
	FormatCSV   OutputFormat = "csv"
)

// TeamOutput represents a team's rating for JSON/CSV output
type TeamOutput struct {
	Rank       int     `json:"rank"`
	TeamID     string  `json:"team_id"`
	TeamName   string  `json:"team_name"`
	MeanELO    float64 `json:"mean_elo"`
	StdDev     float64 `json:"std_dev"`
	Pct5       float64 `json:"percentile_5"`
	Pct25      float64 `json:"percentile_25"`
	Median     float64 `json:"median"`
	Pct75      float64 `json:"percentile_75"`
	Pct95      float64 `json:"percentile_95"`
}

func main() {
	// Command line flags
	dataSource := flag.String("source", "espn", "Data source: 'espn' or 'ncaa'")
	season := flag.Int("season", 2025, "Season year (e.g., 2025 for 2024-2025 season)")
	topN := flag.Int("top", 25, "Number of top teams to display")
	outputFormat := flag.String("format", "table", "Output format: 'table', 'json', or 'csv'")
	outputFile := flag.String("output", "", "Output file (default: stdout)")
	showAll := flag.Bool("all", false, "Show all teams, not just top N")
	teamID := flag.String("team", "", "Show detailed distribution for specific team ID")
	predict := flag.String("predict", "", "Predict matchup: 'teamID1,teamID2'")

	flag.Parse()

	fmt.Println("NCAA Bayesian ELO Rating System")
	fmt.Println("================================")
	fmt.Printf("K Factor: %.2f (optimized via cross-validation)\n", OptimalKFactor)
	fmt.Printf("Season: %d-%d\n", *season-1, *season)
	fmt.Printf("Data Source: %s\n\n", *dataSource)

	// Fetch games
	var games []Game
	var err error

	switch *dataSource {
	case "espn":
		client := NewESPNClient()
		games, err = client.GetSeason(*season)
	case "ncaa":
		client := NewNCAAClient()
		games, err = client.GetSeason(*season)
	default:
		fmt.Fprintf(os.Stderr, "Unknown data source: %s\n", *dataSource)
		os.Exit(1)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "Error fetching games: %v\n", err)
		os.Exit(1)
	}

	// Filter to completed games only
	var completedGames []Game
	for _, g := range games {
		if g.Completed {
			completedGames = append(completedGames, g)
		}
	}

	fmt.Printf("Fetched %d total games, %d completed\n", len(games), len(completedGames))

	if len(completedGames) == 0 {
		fmt.Println("No completed games found. Try a different date range or data source.")
		os.Exit(0)
	}

	// Process games through Bayesian ELO
	elo := NewBayesianELO()
	fmt.Println("Processing games through Bayesian ELO...")
	elo.ProcessGames(completedGames)

	fmt.Printf("Processed %d games for %d teams\n\n", len(elo.GameLog), len(elo.Teams))

	// Handle specific team lookup
	if *teamID != "" {
		elo.PrintTeamDistribution(*teamID)
		return
	}

	// Handle matchup prediction
	if *predict != "" {
		parts := strings.Split(*predict, ",")
		if len(parts) != 2 {
			fmt.Fprintf(os.Stderr, "Invalid predict format. Use: -predict 'teamID1,teamID2'\n")
			os.Exit(1)
		}
		prob, err := elo.PredictMatchup(strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1]))
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error predicting matchup: %v\n", err)
			os.Exit(1)
		}

		team1 := elo.Teams[strings.TrimSpace(parts[0])]
		team2 := elo.Teams[strings.TrimSpace(parts[1])]

		fmt.Printf("Matchup Prediction:\n")
		fmt.Printf("  %s vs %s\n", team1.TeamName, team2.TeamName)
		fmt.Printf("  %s win probability: %.1f%%\n", team1.TeamName, prob*100)
		fmt.Printf("  %s win probability: %.1f%%\n", team2.TeamName, (1-prob)*100)
		return
	}

	// Get rankings
	rankings := elo.GetRankings()

	// Determine how many to show
	showCount := *topN
	if *showAll {
		showCount = len(rankings)
	}
	if showCount > len(rankings) {
		showCount = len(rankings)
	}

	// Prepare output
	var teamOutputs []TeamOutput
	for i := 0; i < showCount; i++ {
		team := rankings[i]
		teamOutputs = append(teamOutputs, TeamOutput{
			Rank:     i + 1,
			TeamID:   team.TeamID,
			TeamName: team.TeamName,
			MeanELO:  team.Dist.Mean(),
			StdDev:   team.Dist.Std(),
			Pct5:     team.Dist.Percentile(5),
			Pct25:    team.Dist.Percentile(25),
			Median:   team.Dist.Percentile(50),
			Pct75:    team.Dist.Percentile(75),
			Pct95:    team.Dist.Percentile(95),
		})
	}

	// Output based on format
	var output string
	switch OutputFormat(*outputFormat) {
	case FormatJSON:
		output = formatJSON(teamOutputs)
	case FormatCSV:
		output = formatCSV(teamOutputs)
	default:
		output = formatTable(teamOutputs, *season)
	}

	// Write output
	if *outputFile != "" {
		err := os.WriteFile(*outputFile, []byte(output), 0644)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error writing output file: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Output written to %s\n", *outputFile)
	} else {
		fmt.Print(output)
	}
}

func formatTable(teams []TeamOutput, season int) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("\nNCAA Men's Basketball Bayesian ELO Rankings (%d-%d Season)\n", season-1, season))
	sb.WriteString(fmt.Sprintf("Generated: %s\n", time.Now().Format("2006-01-02 15:04:05")))
	sb.WriteString(strings.Repeat("=", 100) + "\n")
	sb.WriteString(fmt.Sprintf("%-4s %-30s %8s %8s %8s %8s %8s %8s %8s\n",
		"Rank", "Team", "Mean", "StdDev", "5th%", "25th%", "Median", "75th%", "95th%"))
	sb.WriteString(strings.Repeat("-", 100) + "\n")

	for _, team := range teams {
		sb.WriteString(fmt.Sprintf("%-4d %-30s %8.1f %8.1f %8.1f %8.1f %8.1f %8.1f %8.1f\n",
			team.Rank,
			truncateString(team.TeamName, 30),
			team.MeanELO,
			team.StdDev,
			team.Pct5,
			team.Pct25,
			team.Median,
			team.Pct75,
			team.Pct95))
	}

	sb.WriteString(strings.Repeat("=", 100) + "\n")
	sb.WriteString(fmt.Sprintf("\nNote: ELO distributions show uncertainty in team strength.\n"))
	sb.WriteString(fmt.Sprintf("      Higher StdDev = more uncertainty about true strength.\n"))

	return sb.String()
}

func formatJSON(teams []TeamOutput) string {
	data, _ := json.MarshalIndent(teams, "", "  ")
	return string(data)
}

func formatCSV(teams []TeamOutput) string {
	var sb strings.Builder

	sb.WriteString("rank,team_id,team_name,mean_elo,std_dev,pct_5,pct_25,median,pct_75,pct_95\n")

	for _, team := range teams {
		sb.WriteString(fmt.Sprintf("%d,%s,\"%s\",%.1f,%.1f,%.1f,%.1f,%.1f,%.1f,%.1f\n",
			team.Rank,
			team.TeamID,
			team.TeamName,
			team.MeanELO,
			team.StdDev,
			team.Pct5,
			team.Pct25,
			team.Median,
			team.Pct75,
			team.Pct95))
	}

	return sb.String()
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
