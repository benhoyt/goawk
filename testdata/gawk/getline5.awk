# Message-ID: <4F7832BD.9030709@gmx.com>
# Date: Sun, 01 Apr 2012 11:49:33 +0100
# From: Duncan Moore <duncan.moore@gmx.com>
# To: "bug-gawk@gnu.org" <bug-gawk@gnu.org>
# Subject: [bug-gawk] getline difference from gawk versions >=4.0.0
# 
# Hi
# 
# b.awk:
# 
# BEGIN {
#    system("echo 1 > f")
#    while ((getline a[++c] < "f") > 0) {}
#    print c
# }
# 
# gawk -f b.awk
# 
# Prior to gawk 4.0.0 this outputs:
# 
# 1
# 
# For 4.0.0 and 4.0.1 it outputs:
# 
# 2
# 
# Regards
# Duncan Moore

BEGIN {
    system("echo 1 > f")
    while ((getline a[++c] < "f") > 0) {}
    print c
    system("rm -f f")
}
