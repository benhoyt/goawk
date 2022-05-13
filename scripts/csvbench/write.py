import csv
import sys

writer = csv.writer(sys.stdout)
for i in range(3514073):  # will create a ~1GB file
	writer.writerow([
		i,
		"foo",
		"bob@example.com",
		"simple,quoted",
		"quoted string with \" in it",
		"0123456789",
		"9876543210",
		"The quick brown fox jumps over the lazy dog",
		"",
		"final field",
		i,
		"foo",
		"bob@example.com",
		"simple,quoted",
		"quoted string with \" in it",
		"0123456789",
		"9876543210",
		"The quick brown fox jumps over the lazy dog",
		"",
		"final field",
	])
