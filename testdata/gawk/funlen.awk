# Date: Sat, 15 Mar 2008 16:21:19 +0100
# From: Hermann Peifer <peifer@gmx.net>
# Subject: [Fwd: Gawk length(array) bug]
# To: bug-gawk@gnu.org
# Cc: Aharon Robbins <arnold@skeeve.com>
# Message-id: <47DBE96F.1060406@gmx.net>
# 
# See below. Regards, Hermann
# 
# -------- Original Message --------
# Subject: Re: Gawk length(array) question
# Date: Sat, 15 Mar 2008 08:02:03 -0500
# From: Ed Morton <morton@lsupcaemnt.com>
# Newsgroups: comp.lang.awk
# References: <47DBAE29.4050709@gmx.eu>
# 
# On 3/15/2008 6:08 AM, Hermann Peifer wrote:
# > Hi All,
# > 
# > The Gawk man page says:
# >  > Starting with version 3.1.5, as a non-standard extension,
# >  > with an array  argument, length() returns the number
# >  > of elements in the array.
# > 
# > It looks like Gawk's length(array) extension does not work inside 
# > functions. Is this a bug or feature or am I missing something? See the 
# > example below. I am using GNU Awk 3.1.6
# > 
# > $ cat testdata
# > CD NAME
# > AT Austria
# > BG Bulgaria
# > CH Switzerland
# > DE Germany
# > EE Estonia
# > FR France
# > GR Greece
# > 
# > $ cat test.awk
# > 
# Populate array
NR > 1 { array[$1] = $2 }

# Print array length and call function A
END { print "array:",length(array) ; A(array) }

function A(array_A) { print "array_A:", length(array_A) }
# > 
# > $ gawk -f test.awk testdata
# > array: 7
# > gawk: test.awk:8: (FILENAME=data FNR=8) fatal: attempt to use array 
# > `array_A (from array)' in a scalar context
# > 
# > BTW, there is no such error if I have asort(array_A) or asorti(array_A) 
# > inside the function.
# > 
# > Hermann
# 
# I get the same result with gawk 3.1.6 for cygwin. Obviously you can work
# around
# it since asort() returns the number of elements in an array just like
# length()
# is supposed to (or "for (i in array) lgth++" if you don't want to be
# gawk-specific) but it does seem like a bug. Anyone know if there's a list of
# known gawk bugs on-line somewhere?
# 
# 	Ed.
# 
# 
