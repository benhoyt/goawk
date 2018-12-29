# From sbohdjal@matrox.com  Tue Dec 31 11:41:25 2002
# Return-Path: <sbohdjal@matrox.com>
# X-From_: sbohdjal@matrox.com Mon Dec 30 17:34:41 2002
# Message-Id: <4.3.1.1.20021230101824.00fc4bd8@mailbox.matrox.com>
# Date: Mon, 30 Dec 2002 10:33:10 -0500
# To: bug-gawk@gnu.org
# From: Serge Bohdjalian <sbohdjal@matrox.com>
# Subject: GAWK 3.1.1 bug, DJGPP port
# 
# When I run the following AWK file...
# 
BEGIN {
    $0 = "00E0";
    print $0 ", " ($0 && 1) ", " ($0 != "");
    $1 = "00E0";
    print $1 ", " ($1 && 1) ", " ($1 != "");
}
# 
# With the SimTel version of GAWK 3.1.1 for Windows (downloadable from 
# ftp://ftp.cdrom.com/pub/simtelnet/gnu/djgpp/v2gnu/), I get the following 
# output...
# 
# 00E0, 0, 1
# 00E0, 1, 1
# 
# With the Cygwin version of GAWK 3.1.1 for Windows, I get...
# 
# 00E0, 1, 1
# 00E0, 1, 1
# 
# As far as I know, if "$0" isn't blank, the value of "($0 && 1)" should be 
# "1" (true). I get the same problem if I substitute "00E0" with "00E1" to 
# "00E9". Other strings don't have have this problem (for example, "00EA"). 
# The problem occurs whether I use file input or whether I manually assign 
# "$0" (as above).
# 
# The problem is also discussed in a comp.lang.awk posting ("Bug in GAWK 
# 3.1.1?", Dec. 27, 2002).
# 
# -Serge
