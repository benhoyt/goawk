{
	x = "0x" $1 ; print x, x + 0
	for (i=1; i<=NF; i++)
		if ($i) print $i, "is not zero"
}
