# test whether --lint catches uninitialized fields:
function pr()
{
	print
}

BEGIN {
	pr()
	print $0
	print $(1-1)
	print $1
	NF=3; print $2
}
