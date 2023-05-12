BEGIN {
	abc(2)
}


function abc(c, A, B)
{
	print "abc(" c ", " length(A) ")"
	if (! c--) {
		return 
	}
	B[""]
	print length(B)
	return abc(c, B)
}
