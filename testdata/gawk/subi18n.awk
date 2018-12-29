# Date: Mon, 27 Feb 2006 12:35:30 +0900
# From: KIMURA Koichi <kimura.koichi@canon.co.jp>
# Subject: gawk: sub_common has multi-byte aware bug
# To: bug-gawk@gnu.org
# Message-id: <20060227121045.2198.KIMURA.KOICHI@canon.co.jp>
# 
# Hi,
# 
# A certain user faced bug of sub builtin function and report to me.
# Then I investigated the bug.
# 
# reproduce script is here.

BEGIN {
	str = "type=\"directory\" version=\"1.0\""
	#print "BEGIN:", str

	while (str) {
		sub(/^[^=]*/, "", str);
		s = substr(str, 2)
		print s
		sub(/^="[^"]*"/, "", str)
		sub(/^[ \t]*/, "", str)
	}
}

# and sample result is here (on GNU/Linux Fedora core 3)
# 
# [kbk@skuld gawk-3.1.5]$ LC_ALL=C ./gawk -f subbug.awk
# "directory" version="1.0"
# "1.0"
# [kbk@skuld gawk-3.1.5]$ LC_ALL=en_US.UTF-8 ./gawk -f subbug.awk
# "directory" version="1.0"
# "dire
# [kbk@skuld gawk-3.1.5]$
# 
# In my investigation, this bug is cause by don't release wide-string when
# sub is executed.
# 
# patch is here.
# 
# --- builtin.c.orig	2005-07-27 03:07:43.000000000 +0900
# +++ builtin.c	2006-02-26 02:07:52.000000000 +0900
# @@ -2463,6 +2468,15 @@ sub_common(NODE *tree, long how_many, in
#  	t->stptr = buf;
#  	t->stlen = textlen;
# 
# +#ifdef MBS_SUPPORT
# +    if (t->flags & WSTRCUR) {
# +        if (t->wstptr != NULL)
# +            free(t->wstptr);
# +        t->wstptr = NULL;
# +        t->wstlen = 0;
# +        t->flags &= ~WSTRCUR;
# +    }
# +#endif
#  	free_temp(s);
#  	if (matches > 0 && lhs) {
#  		if (priv) {
# 
# 
# -- 
# KIMURA Koichi
# 
# 
# #####################################################################################
# This Mail Was Scanned by 012.net AntiVirus Service1- Powered by TrendMicro Interscan
# 
