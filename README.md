# NCAA-Bayes-ELO

A Bayesian ELO rating system for NCAA Men's Basketball that maintains full probability distributions over team strengths rather than point estimates.

## Current Rankings (2024-25 Season)

![ELO Distributions](elo_distributions.png)

The chart above shows the probability distributions for the top 20 teams. Key observations:
- **Florida** has the highest mean ELO (~2200) and a relatively narrow distribution
- **Villanova** (#16) and **Kansas State** (#19) have wider distributions due to more variable results
- Teams with narrower distributions (Tennessee, Alabama) have more consistent performance

## Features

- **Bayesian ELO**: Maintains probability distributions over team ELO ratings, quantifying uncertainty
- **Optimized Parameters**: Uses K factor of 0.90, cross-validated on 2024 season data (see [ELO-Tuning-Go](https://github.com/corykiser/ELO-Tuning-Go))
- **Dual Data Sources**: Fetches game data from ESPN or NCAA.com APIs
- **Multiple Output Formats**: Table, JSON, or CSV output
- **Matchup Predictions**: Predict win probabilities for any two teams

## Quick Start

```bash
# Build
go build -o ncaa-bayes-elo

# Run with default settings (ESPN data, 2025 season, top 25)
./ncaa-bayes-elo

# Use NCAA.com data source
./ncaa-bayes-elo -source ncaa

# Show top 50 teams
./ncaa-bayes-elo -top 50

# Output as JSON
./ncaa-bayes-elo -format json -output rankings.json

# Output as CSV
./ncaa-bayes-elo -format csv -output rankings.csv

# Show all teams
./ncaa-bayes-elo -all

# Get detailed distribution for a specific team
./ncaa-bayes-elo -team "57"  # Team ID from ESPN

# Predict a matchup
./ncaa-bayes-elo -predict "57,150"  # Florida vs Duke
```

## Command Line Options

| Flag | Default | Description |
|------|---------|-------------|
| `-source` | `espn` | Data source: `espn` or `ncaa` |
| `-season` | `2025` | Season year (e.g., 2025 = 2024-25 season) |
| `-top` | `25` | Number of top teams to display |
| `-all` | `false` | Show all teams |
| `-format` | `table` | Output format: `table`, `json`, or `csv` |
| `-output` | stdout | Output file path |
| `-team` | | Show detailed distribution for team ID |
| `-predict` | | Predict matchup: `teamID1,teamID2` |

## Sample Output

```
NCAA Men's Basketball Bayesian ELO Rankings (2024-2025 Season)
Generated: 2025-12-06 16:34:53
====================================================================================================
Rank Team                               Mean   StdDev     5th%    25th%   Median    75th%    95th%
----------------------------------------------------------------------------------------------------
1    Florida Gators                   2198.3     98.3   2045.0   2130.0   2195.0   2260.0   2365.0
2    Houston Cougars                  2104.4     94.8   1955.0   2040.0   2100.0   2165.0   2265.0
3    Auburn Tigers                    2098.1     89.4   1955.0   2035.0   2095.0   2155.0   2250.0
4    Duke Blue Devils                 2049.5    106.8   1880.0   1975.0   2045.0   2120.0   2230.0
5    Tennessee Volunteers             2028.4     85.9   1890.0   1970.0   2025.0   2085.0   2170.0
...
```

## Understanding the Output

- **Mean**: Expected ELO rating (higher = better)
- **StdDev**: Uncertainty in the rating (higher = less certain)
- **Percentiles**: Distribution of possible true ELO values
  - 5th%: Conservative lower bound
  - 95th%: Optimistic upper bound
  - 50% (Median): Most likely true rating

Teams with high StdDev have more uncertain ratings, often due to fewer games played or inconsistent results.

## Data Sources

### ESPN API (Default)
- Undocumented but reliable JSON API
- No authentication required
- Rate limited by politeness (100ms delay between requests)

### NCAA.com API
- Community wrapper by [henrygd](https://github.com/henrygd/ncaa-api)
- Covers all NCAA sports
- Rate limited to 5 req/sec

## How It Works

### Bayesian ELO Algorithm

1. **Initialize**: Each team starts with a normal prior distribution (mean=1500, std=300)

2. **For each game**:
   - Compute joint distribution of both teams' ELOs
   - Calculate likelihood: `P(winner wins | ELO_diff) = 1 / (1 + 10^(-diff * K / 400))`
   - Multiply joint by likelihood and normalize
   - Marginalize to get updated distributions

3. **Output**: Full probability distributions showing uncertainty

### Why K = 0.90?

The K factor was optimized via cross-validation:
- **Training**: 2016-2019, 2022-2023 seasons
- **Testing**: 2024 season
- **Result**: K=0.90 achieves 90% better log loss than K=30.464

A lower K factor produces better-calibrated probabilities by avoiding overconfident predictions.

## Project Structure

```
ncaa-bayes-elo-go/
├── main.go           # CLI and output formatting
├── bayesian_elo.go   # Core Bayesian ELO algorithm
├── espn_client.go    # ESPN API client
├── ncaa_client.go    # NCAA API client
├── go.mod
└── README.md
```

## Related Projects

- [ELO-Tuning-Go](https://github.com/corykiser/ELO-Tuning-Go): Parameter optimization for this system

## License

MIT
