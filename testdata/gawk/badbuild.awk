$1 == $2 == $3 {
	print "Gawk was built incorrectly."
	print "Use bison, not byacc or something else!"
	print "(Really, why aren't you using the awkgram.c in the distribution?)"
	exit 42
}
