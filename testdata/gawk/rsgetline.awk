# Date: Sun, 4 May 2014 18:09:01 +0200
# From: Davide Brini <dave_br@gmx.com>
# To: bug-gawk@gnu.org
# Subject: Re: [bug-gawk] Computed regex and getline bug / issue
# 
# I have been able to reduce the behavior to these simple test cases, which
# (unless I'm missing something obvious) should behave identically but don't:
# 
# $ printf '1,2,' | gawk 'BEGIN{RS="[,]+"}{print; a = getline; print "-"a"-"; print}'
# 1
# -0-
# 1

BEGIN {
	RS = "[,]+"
}

{
	printf "[%s] [%s]\n", $0, RT
	a = getline
	print "-"a"-"
	printf "[%s] [%s]\n", $0, RT
}
