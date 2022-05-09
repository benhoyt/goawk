import csv
import sys

writer = csv.writer(sys.stdout)
for i in range(10000000):
	writer.writerow([i, "foo", "bob@example.com", "quoted,string", "final field"])
