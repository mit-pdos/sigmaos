import matplotlib.pyplot as plt
from collections import Counter
import sys

if len(sys.argv) != 3:
    print("Usage: python3 plot_stats.py <input_file> <output_file>")
    sys.exit(1)

input_file = sys.argv[1]
output_file = sys.argv[2]

# Read and round float values from input file
with open(input_file) as f:
    values = [round(float(line.strip()), 2) for line in f if line.strip()]

# Count frequencies
counts = Counter(values)
diffs = sorted(counts.keys())
freqs = [counts[d] for d in diffs]

# Plot
plt.figure(figsize=(8, 4))
plt.bar(diffs, freqs, width=0.5)
plt.xlabel("Difference (array[5] - array[2])")
plt.ylabel("Frequency")
plt.title("Frequency of Differences")
plt.tight_layout()

# Save to output file
plt.savefig(output_file)