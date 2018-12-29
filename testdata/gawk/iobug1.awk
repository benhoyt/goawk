# From arnold@f7.net  Fri Nov 26 11:53:12 2004
# X-Envelope-From: james@nocrew.org
# X-Envelope-To: <arnold@skeeve.com>
# To: bug-gawk@gnu.org
# Subject: gawk 3.1.4: reproducible hang, regression from 3.1.3
# From: James Troup <james@nocrew.org>
# Date: Fri, 26 Nov 2004 03:14:05 +0000
# Message-ID: <877jo9qp36.fsf@shiri.gloaming.local>
# User-Agent: Gnus/5.1006 (Gnus v5.10.6) Emacs/21.3 (gnu/linux)
# MIME-Version: 1.0
# Content-Type: text/plain; charset=us-ascii
# 
# 
# Hi,
# 
# A Debian user reported[0] gawk 3.1.4 broke a (relatively) complex
# program that makes extensive use of awk, called 'apt-move'.  I finally
# managed to reduced the problem down to a 3 line test case, enclosed
# below[1].
# 
# I believe the problem comes from the following code, introduced in
# 3.1.4:
# 
# [io.c, line 560]
# | 	for (rp = red_head; rp != NULL; rp = rp->next) {
# | 		if ((rp->flag & RED_EOF) && tree->type == Node_redirect_pipein) {
# | 			if (rp->pid != -1)
# | 				wait_any(0);
# | 		}
# 
# The problem is that, if we have an existing redirect which is a simple
# file redirect[b] and it's hit EOF and we try to create a new '|'
# redirect[c], this new code will try to wait(2) and if there are any
# other redirects which _did_ spawn a child (like [a]) the wait() will
# hang indefinitely waiting for it to exit.
# 
# Hope that makes sense :)
# 
# -- 
# James
# 
# [0] http://bugs.debian.org/263964
# 
# [1] 
# ================================================================================
#!/usr/bin/gawk -f

BEGIN {
	printf "" | "cat"             # [a]
	getline line < "/dev/null"    # [b]
	"true" | getline line         # [c]
}
# ================================================================================
