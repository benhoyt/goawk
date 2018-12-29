# Date: Sun, 28 May 2006 11:20:58 +0200
# From: Frantisek Hanzlik <franta@hanzlici.cz>
# Subject: sub() function do'nt alter string length in awk 3.1.5
# To: bug-gawk@gnu.org
# Message-id: <44796B7A.3050908@hanzlici.cz>
# 
# Hello,
# I not know when it is my mistake or gawk bug - in simple example below
# I delete some chars from string variable, and after this string is
# modified, but its length is unchanged.
# 
# awk 'BEGIN{A="1234567890abcdef";
#   for (i=1;i<6;i++){print length(A),"A=" A ".";sub("....","",A)}
# }'
# 16 A=1234567890abcdef.
# 16 A=567890abcdef.
# 16 A=90abcdef.
# 16 A=cdef.
# 16 A=.
# 
# When I use gensub() instead of sub(), result is as I expected:
# 
# awk 'BEGIN{A="1234567890abcdef";
#   for (i=1;i<6;i++){print length(A),"A=" A ".";A=gensub("....","",1,A)}
# }'
# 16 A=1234567890abcdef.
# 12 A=567890abcdef.
# 8 A=90abcdef.
# 4 A=cdef.
# 0 A=.
# 
# OS/GAWK versions:
# - GNU/Linux kernel 2.6.16-1.2122_FC5 #1 i686, Fedora Core 5 distro
# - glibc-2.4-8
# - GNU Awk 3.1.5
# 
# Yours sincerely
# Frantisek Hanzlík
# 
# == Lucní 502        Linux/Unix, Novell, Internet   Tel: +420-373729699 ==
# == 33209 Stenovice   e-mail:franta@hanzlici.cz     Fax: +420-373729699 ==
# == Czech Republic        http://hanzlici.cz/       GSM: +420-604117319 ==
# 
# 
# 
# #####################################################################################
# This Mail Was Scanned by 012.net AntiVirus Service3- Powered by TrendMicro Interscan
# 
BEGIN{A="1234567890abcdef";
   for (i=1;i<6;i++){print length(A),"A=" A ".";sub("....","",A)}
}
BEGIN{A="1234567890abcdef";
   for (i=1;i<6;i++){print length(A),"A=" A ".";A=gensub("....","",1,A)}
}
