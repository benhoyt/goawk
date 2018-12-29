# Date: Fri, 06 Jan 2006 14:02:17 -0800
# From: Paul Eggert <eggert@CS.UCLA.EDU>
# Subject: gawk misparses $expr++ if expr ends in ++
# To: bug-gawk@gnu.org
# Message-id: <87irsxypzq.fsf@penguin.cs.ucla.edu>
# 
# Here's an example of the problem:
# 
# $ gawk 'BEGIN{a=3}{print $$a++++}'
# gawk: {print $$a++++}
# gawk:               ^ syntax error
# 
# But it's not a syntax error, as the expression conforms to the POSIX
# spec: it should be treated like '$($a++)++'.
# 
# Mawk, Solaris awk (old awk), and Solaris nawk all accept the
# expression.  For example:
# 
# $ echo '3 4 5 6 7 8 9' | nawk 'BEGIN{a=3}{print $$a++++}'
# 7
# 
# This is with gawk 3.1.5 on Solaris 8 (sparc).
# 
# 
# #####################################################################################
# This Mail Was Scanned by 012.net AntiVirus Service1- Powered by TrendMicro Interscan
# 
BEGIN { a = 3 }

{
	print "in:", $0
	print "a =", a
	print $$a++++
	print "out:", $0
}
