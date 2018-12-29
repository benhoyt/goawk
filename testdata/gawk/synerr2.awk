# From: =?ISO-8859-1?Q?J=FCrgen_Kahrs?= <Juergen.KahrsDELETETHIS@vr-web.de>
# Newsgroups: gnu.utils.bug
# Subject: Re: gawk-3.1.5: syntax error, core dump
# Date: Fri, 23 Jun 2006 18:12:07 +0200
# Lines: 12
# Approved: bug-gnu-utils@gnu.org
# Message-ID: <mailman.3258.1151079135.9609.bug-gnu-utils@gnu.org>
# References: <mailman.3236.1151045898.9609.bug-gnu-utils@gnu.org>
# Reply-To: Juergen.KahrsDELETETHIS@vr-web.de
# NNTP-Posting-Host: lists.gnu.org
# Mime-Version: 1.0
# Content-Type: text/plain; charset=ISO-8859-1
# Content-Transfer-Encoding: 7bit
# X-Trace: news.Stanford.EDU 1151079136 27033 199.232.76.165 (23 Jun 2006 16:12:16 GMT)
# X-Complaints-To: news@news.stanford.edu
# To: gnu-utils-bug@moderators.isc.org
# Envelope-to: bug-gnu-utils@gnu.org
# X-Orig-X-Trace: individual.net
# 	vYX9N7nUUtqHxPyspweN0gZ4Blkl17z/xU01EwbykxB178O8M=
# User-Agent: Thunderbird 1.5 (X11/20060317)
# In-Reply-To: <mailman.3236.1151045898.9609.bug-gnu-utils@gnu.org>
# X-BeenThere: bug-gnu-utils@gnu.org
# X-Mailman-Version: 2.1.5
# Precedence: list
# List-Id: Bug reports for the GNU utilities <bug-gnu-utils.gnu.org>
# List-Unsubscribe: <http://lists.gnu.org/mailman/listinfo/bug-gnu-utils>,
# 	<mailto:bug-gnu-utils-request@gnu.org?subject=unsubscribe>
# List-Archive: <http://lists.gnu.org/pipermail/bug-gnu-utils>
# List-Post: <mailto:bug-gnu-utils@gnu.org>
# List-Help: <mailto:bug-gnu-utils-request@gnu.org?subject=help>
# List-Subscribe: <http://lists.gnu.org/mailman/listinfo/bug-gnu-utils>,
# 	<mailto:bug-gnu-utils-request@gnu.org?subject=subscribe>
# Path: news.012.net.il!seanews2.seabone.net!newsfeed.albacom.net!news.mailgate.org!newsfeed.stueberl.de!newsfeed.news2me.com!headwall.stanford.edu!newsfeed.stanford.edu!shelby.stanford.edu!individual.net!not-for-mail
# Xref: news.012.net.il gnu.utils.bug:813
# 
# Karel Zak wrote:
# 
# >  it seems that gawk has problem with "syntax error" reporting:
# > 
# >  ./gawk '/^include / { system(sprintf("cd /etc; cat %s", [$]2)); skip
# >  = 1; } { if (!skip) print $0; skipQuit; }' < /etc/ld.so.conf 
# 
# This test case can be boiled down to
# 
#   gawk 'BEGIN {sprintf("%s", $)}'
# 
BEGIN { sprintf("%s", $) }
