BEGIN {
	FS = "(^x+)|( +)"
}
	
{
	for (i = 1; i <= NF; i++)
		printf "-->%s<--\n", $i
}
