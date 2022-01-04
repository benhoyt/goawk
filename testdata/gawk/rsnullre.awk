BEGIN {
	RS = "()"
}

{ printf("<<%s>>\n", $0) ; printf("<%s>\n", RT) }
