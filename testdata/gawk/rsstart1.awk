# From arnold@f7.net  Wed Dec 15 11:32:46 2004
# Date: Tue, 14 Dec 2004 14:48:58 +0100
# From: Stepan Kasal <kasal@ucw.cz>
# Subject: gawk bug with RS="^..."
# To: bug-gawk@gnu.org
# Message-id: <20041214134858.GA15490@matsrv.math.cas.cz>
# 
# Hello,
#   I've noticed a problem with "^" in RS in gawk.  In most cases, it seems
# to match only the beginning of the file.  But in fact it matches the
# beginning of gawk's internal buffer.
# 
# Observe the following example:
# 
# $ gawk 'BEGIN{for(i=1;i<=100;i++) print "Axxxxxx"}' >file
# $ gawk 'BEGIN{RS="^A"} END{print NR}' file
# 2
# $ gawk 'BEGIN{RS="^Ax*\n"} END{print NR}' file
# 100
# $ head file | gawk 'BEGIN{RS="^Ax*\n"} END{print NR}'
# 10
# $
# 
# I think this calls for some clarification/fix.  But I don't have any
# fixed opinion how the solution should look like.
# 
# Have a nice day,
#         Stepan Kasal
# 
# PS: See also the discussion of the issue in the comp.lang.awk newsgroup.
BEGIN { RS = "^A" }
END { print NR }
