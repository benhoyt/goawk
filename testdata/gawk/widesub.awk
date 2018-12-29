BEGIN {
	str = "type=\"directory\" version=\"1.0\""
	#print "BEGIN:", str

	while (str) {
		sub(/^[^=]*/, "", str);
		s = substr(str, 2)
		print s
		sub(/^="[^"]*"/, "", str)
		sub(/^[ \t]*/, "", str)
	}
}
