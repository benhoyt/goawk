import csv
import sys

lines, fields = 0, 0
for row in csv.reader(sys.stdin):
	lines += 1
	fields += len(row)

print(lines, fields)
