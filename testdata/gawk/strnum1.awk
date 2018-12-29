# Date: Tue, 04 Jul 2006 21:06:14 +0200 (MEST)
# From: Heiner Marxen <Heiner.Marxen@DrB.Insel.DE>
# Subject: conversion error
# To: bug-gawk@gnu.org
# Message-id: <200607041906.k64J6Eqa019360@drb9.drb.insel.de>
# 
# Hello,
# 
# The following awk script fails for gawk 3.1.4 and 3.1.5.
# Older versions did not do this, but I cannot say, how old they were.
# 
BEGIN {
    if( 0 ) {		#ok
	t = "8"
    }else {		#fails
	t = ""
	t = t "8"
    }
    printf("8  = %d\n", 0+t)	# ok without this line
    t = t "8"			# does not invalidate numeric interpretation
    printf("88 = %s\n", 0+t)
    ## The above prints "88 = 8" with gawk 3.1.4 and 3.1.5
}
# 
# 
# The following one-liner already exhibits the bug:
# 
# gawk 'BEGIN{t=""; t=t "8";printf("8=%d\n", 0+t);t=t "8";printf("88=%s\n", 0+t)}'
# 
# 
# Preliminary observation: under somewhat strange conditions a variable
# does retain its numeric interpretation although something is appended to it.
# -- 
# Heiner Marxen				http://www.drb.insel.de/~heiner/
# 
