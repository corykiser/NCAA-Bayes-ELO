# PyMC NCAA Basketball ELO Implementation Prompt

## Task Overview

Create a PyMC-based Bayesian ELO system for NCAA Men's Basketball that produces **joint posterior samples** representing coherent "possible worlds" of team ratings. This addresses a key limitation of the existing Go implementation which only stores independent marginal distributions.

## Existing Implementation Reference

The current system is implemented in Go in this repository:

```
/home/user/NCAA-Bayes-ELO/
├── bayesian_elo.go    # Core Bayesian inference (discrete distributions)
├── main.go            # CLI interface and output formatting
├── espn_client.go     # ESPN API client for fetching games
├── ncaa_client.go     # NCAA API client (alternative source)
├── cache.go           # Local JSON caching layer
└── generate_plot.py   # Matplotlib visualization
```

### Current Approach (Go)
The existing system uses **discrete exact Bayesian inference**:
- Each team's ELO is represented as a discrete distribution (0-3000, step=5)
- After each game, it computes the joint distribution of the two teams, applies the likelihood, then marginalizes back to independent distributions
- **Key limitation**: Correlations between teams are lost after marginalization

### What We Want (PyMC)
A system that:
1. Models all team strengths as latent variables in a single probabilistic model
2. Uses MCMC to sample from the **joint posterior** of all team ratings
3. Each sample is a complete, internally consistent set of ratings for all ~360 teams
4. Enables bracket simulations where team ratings respect the evidence structure

## Data Sources

### ESPN API (Primary)
```
Base URL: https://site.api.espn.com/apis/site/v2/sports/basketball/mens-college-basketball
Endpoint: /scoreboard?dates=YYYYMMDD&limit=500

Response structure:
{
  "events": [{
    "id": "401234567",
    "date": "2024-11-15T00:00Z",
    "competitions": [{
      "neutralSite": false,
      "competitors": [
        {
          "homeAway": "home",
          "winner": true,
          "team": {"id": "150", "displayName": "Duke Blue Devils"},
          "score": "85"
        },
        {
          "homeAway": "away",
          "winner": false,
          "team": {"id": "153", "displayName": "North Carolina Tar Heels"},
          "score": "78"
        }
      ],
      "status": {"type": {"completed": true}}
    }]
  }]
}
```

### NCAA API (Alternative)
```
Base URL: https://ncaa-api.henrygd.me
Endpoint: /scoreboard/basketball-men/d1/YYYY/MM/DD

Response structure:
{
  "games": [{
    "game": {
      "gameID": "5678",
      "gameState": "final",
      "home": {
        "names": {"full": "Duke Blue Devils"},
        "score": "85",
        "winner": true,
        "teamId": "150"
      },
      "away": {
        "names": {"full": "North Carolina Tar Heels"},
        "score": "78",
        "winner": false,
        "teamId": "153"
      }
    }
  }]
}
```

### Season Definition
- Season "2025" = November 1, 2024 → April 15, 2025
- Fetch all dates in range, ~150 days of games
- Rate limit: ESPN ~10 req/sec, NCAA ~5 req/sec

## PyMC Model Specification

### Core Model Structure
```python
import pymc as pm
import numpy as np

with pm.Model() as elo_model:
    # Prior for each team's strength
    # ~360 teams, each with a latent ELO rating
    team_elo = pm.Normal('team_elo', mu=1500, sigma=300, shape=n_teams)

    # For each game, the win probability is:
    # P(home wins) = sigmoid((elo_home - elo_away) * K / 400)
    # where K = 0.90 (tuned parameter from Go implementation)

    elo_diff = team_elo[home_team_idx] - team_elo[away_team_idx]

    # Likelihood for observed game outcomes
    # outcome = 1 if home team won, 0 if away team won
    pm.Bernoulli('outcomes',
                 logit_p=elo_diff * 0.90 / 400,
                 observed=game_outcomes)
```

### Key Parameters (from Go implementation)
```python
K_FACTOR = 0.90        # Likelihood sensitivity (cross-validated)
PRIOR_MEAN = 1500.0    # Prior mean ELO
PRIOR_STD = 300.0      # Prior standard deviation
ELO_MIN = 0.0          # For reference only (soft constraint via prior)
ELO_MAX = 3000.0       # For reference only
```

## Implementation Requirements

### 1. Data Loading Module
Create a Python module that:
- Fetches game data from ESPN API (primary) or NCAA API (fallback)
- Implements parallel fetching with rate limiting
- Caches completed seasons locally (JSON format)
- Handles the existing cache format for compatibility

```python
# Desired interface
games = load_season(2025, source='espn')  # Returns list of Game objects

class Game:
    date: datetime
    home_team_id: str
    home_team_name: str
    away_team_id: str
    away_team_name: str
    home_score: int
    away_score: int
    neutral_site: bool
    completed: bool
```

### 2. Model Building Module
- Create team ID → index mapping
- Build PyMC model with proper indexing
- Handle the constraint that each game involves exactly 2 teams

### 3. Inference Module
- Run NUTS sampler (or alternative like ADVI for speed)
- Suggested: 4 chains, 1000 draws each, 500 tune
- Store posterior samples efficiently (NetCDF via ArviZ)

### 4. Analysis Module
Output capabilities:
- **Rankings**: Mean, std, percentiles for each team (like Go version)
- **Joint samples**: Export samples as CSV/parquet for bracket simulations
- **Matchup predictions**: P(A beats B) integrating over joint uncertainty
- **Diagnostics**: R-hat, ESS, trace plots

### 5. Bracket Simulation
The key new capability:
```python
def simulate_bracket(posterior_samples, bracket_structure):
    """
    For each posterior sample:
      - Extract that sample's team ratings
      - Simulate entire bracket using those ratings
      - Record winner

    Returns distribution over tournament winners that respects
    correlations between team strengths.
    """
```

## Output Format

### Rankings Table (match Go format)
```
Rank Team                  Mean   StdDev   5th%   25th%  Median  75th%  95th%
1    Duke Blue Devils    2099.2   183.7  1810.0  1970.0 2090.0  2220.0 2415.0
2    Iowa State Cyclones 2070.1   188.1  1775.0  1940.0 2065.0  2190.0 2390.0
...
```

### Joint Samples Export
```csv
sample_id,Duke,North_Carolina,Kansas,Kentucky,...
0,2105,1987,2034,1923,...
1,1892,1845,1901,1756,...
...
```

Each row is a coherent "possible world" where all ratings are consistent with game evidence.

## Project Structure

```
pymc_elo/
├── __init__.py
├── data/
│   ├── __init__.py
│   ├── espn_client.py      # ESPN API fetching
│   ├── ncaa_client.py      # NCAA API fetching
│   └── cache.py            # Local caching
├── model/
│   ├── __init__.py
│   ├── elo_model.py        # PyMC model definition
│   └── inference.py        # MCMC sampling
├── analysis/
│   ├── __init__.py
│   ├── rankings.py         # Generate rankings tables
│   ├── matchups.py         # Matchup predictions
│   └── brackets.py         # Bracket simulation
├── cli.py                  # Command-line interface
└── requirements.txt
```

## Dependencies
```
pymc>=5.0
arviz
numpy
pandas
requests
aiohttp  # For parallel fetching
rich     # For nice CLI output (optional)
```

## CLI Interface

```bash
# Fetch data and run inference
python -m pymc_elo fit --season 2025 --draws 2000 --chains 4

# Generate rankings
python -m pymc_elo rankings --season 2025 --top 50

# Export samples for bracket simulation
python -m pymc_elo export-samples --season 2025 --output samples.parquet

# Predict matchup
python -m pymc_elo predict "Duke" "North Carolina"

# Simulate bracket (reads bracket structure from file)
python -m pymc_elo simulate-bracket bracket_2025.json --simulations 10000
```

## Important Considerations

### Computational Cost
- ~360 teams × ~5000 games = large model
- MCMC will be slow (minutes to hours, not seconds)
- Consider:
  - Starting with ADVI for quick approximation
  - Using JAX backend (NumPyro) for GPU acceleration
  - Saving/loading traces to avoid re-running

### Model Extensions to Consider (optional)
- Home court advantage parameter
- Conference-level hierarchical structure
- Time-varying ratings (teams improve/decline)
- Margin of victory (use score difference, not just win/loss)

### Validation
- Compare rankings to Go implementation (should be similar for means)
- Check that joint samples produce sensible bracket simulations
- Verify convergence diagnostics (R-hat < 1.01, ESS > 400)

## Why This Matters

The Go implementation produces independent marginals. When you sample:
```
Duke ~ N(2000, 180)
UNC ~ N(1950, 175)
```

You might get Duke=1800, UNC=2100 in the same "simulation" - but this is inconsistent with the game evidence (Duke beat UNC).

With PyMC joint sampling:
```
Sample 1: Duke=2050, UNC=1980  (Duke slightly better)
Sample 2: Duke=1850, UNC=1800  (both weak, but Duke still ahead)
```

Every sample respects the constraint that Duke > UNC (on average) because Duke won their game.

This produces **more realistic bracket simulations** because connected teams move together.
