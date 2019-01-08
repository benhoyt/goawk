BEGIN {
	abc(2)
}


function array_length(a,   k, n) {
    n = 0
    for (k in a) n++
    return n
}

function abc(c, A, B)
{
	print "abc(" c ", " array_length(A) ")"
	if (!c) {
		return 
	}
    c--
	B[""] = 1
	print array_length(B)
	return abc(c, B)
}
