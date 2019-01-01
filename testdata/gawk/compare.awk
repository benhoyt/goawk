BEGIN {
	if ("0") print "zero"
	if ("") print "null"
	if (0) print 0
}
{
	if ($0) print $0
	if ($1) print $1
}
