#!/usr/bin/env python3
"""Generate probability distribution plot for top 20 NCAA basketball teams."""

import json
import numpy as np
import matplotlib.pyplot as plt
from scipy import stats

# Load team data
with open('top20.json', 'r') as f:
    teams = json.load(f)

# Create figure with good size for README
fig, ax = plt.subplots(figsize=(14, 10))

# Color palette - using distinct colors that work well together
# Split into warm and cool colors for better differentiation
colors = [
    '#1f77b4',  # blue
    '#ff7f0e',  # orange
    '#2ca02c',  # green
    '#d62728',  # red
    '#9467bd',  # purple
    '#8c564b',  # brown
    '#e377c2',  # pink
    '#7f7f7f',  # gray
    '#bcbd22',  # olive
    '#17becf',  # cyan
    '#aec7e8',  # light blue
    '#ffbb78',  # light orange
    '#98df8a',  # light green
    '#ff9896',  # light red
    '#c5b0d5',  # light purple
    '#c49c94',  # light brown
    '#f7b6d2',  # light pink
    '#c7c7c7',  # light gray
    '#dbdb8d',  # light olive
    '#9edae5',  # light cyan
]

# Line styles for additional differentiation
line_styles = ['-', '--', '-.', ':']

# X-axis range (ELO values) - adjusted for early season with higher uncertainty
x = np.linspace(1200, 2600, 500)

# Plot each team's distribution
for i, team in enumerate(teams):
    mean = team['mean_elo']
    std = team['std_dev']

    # Calculate normal distribution PDF
    pdf = stats.norm.pdf(x, mean, std)

    # Normalize for visibility (scale by std so all curves have similar peak heights)
    pdf_scaled = pdf * std * 2.5

    # Get color and line style
    color = colors[i % len(colors)]
    linestyle = line_styles[(i // 10) % len(line_styles)]

    # Use thicker lines for top 6 teams
    linewidth = 2.5 if i < 6 else 1.8
    alpha = 0.9 if i < 6 else 0.7

    # Short name for legend
    name = team['team_name'].replace(' Crimson Tide', '').replace(' Volunteers', '')
    name = name.replace(' Blue Devils', '').replace(' Spartans', '').replace(' Cougars', '')
    name = name.replace(' Wildcats', '').replace(' Red Storm', '').replace(' Tigers', '')
    name = name.replace(' Terrapins', '').replace(' Owls', '').replace(' Wolverines', '')
    name = name.replace(' Aggies', '').replace(' Razorbacks', '').replace(' Rebels', '')
    name = name.replace(' Red Raiders', '').replace(' Gators', '')

    label = f"{team['rank']}. {name}"

    ax.plot(x, pdf_scaled, color=color, linestyle=linestyle,
            linewidth=linewidth, alpha=alpha, label=label)

    # Add subtle fill for top 3 teams
    if i < 3:
        ax.fill_between(x, pdf_scaled, alpha=0.1, color=color)

# Styling
ax.set_xlabel('ELO Rating', fontsize=14, fontweight='bold')
ax.set_ylabel('Probability Density (scaled)', fontsize=14, fontweight='bold')
ax.set_title('NCAA Men\'s Basketball - Bayesian ELO Distributions (2025-26 Season)\nTop 20 Teams',
             fontsize=16, fontweight='bold', pad=20)

# Legend - two columns for readability
legend = ax.legend(loc='upper left', fontsize=9, ncol=2, framealpha=0.95,
                   title='Rank. Team', title_fontsize=10)

# Grid
ax.grid(True, alpha=0.3, linestyle='-')
ax.set_axisbelow(True)

# Set axis limits
ax.set_xlim(1200, 2600)
ax.set_ylim(0, ax.get_ylim()[1] * 1.05)

# Add annotation explaining the chart
ax.annotate('Wider distributions = more uncertainty\nNarrower distributions = more confidence in rating',
            xy=(0.98, 0.98), xycoords='axes fraction',
            ha='right', va='top', fontsize=10,
            bbox=dict(boxstyle='round,pad=0.5', facecolor='wheat', alpha=0.8))

plt.tight_layout()
plt.savefig('elo_distributions.png', dpi=150, bbox_inches='tight',
            facecolor='white', edgecolor='none')
print("Saved elo_distributions.png")

plt.close()
