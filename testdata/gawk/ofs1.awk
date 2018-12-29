# Translate this shell script into gawk:
#
#! /bin/sh -
# 
# awktest()
# {
#   echo a:b:c | $AWK -F":" '{$2="x"; OFS=FS; print}'
#   echo a:b:c | $AWK -F":" '{$2="x"; print; OFS=FS; print}'
#   echo a:b:c | $AWK -F":" '{$2="x"; print $1; OFS=FS; print}'
#   echo a:b:c | $AWK -F":" '{$2="x"; print; $2=$2; OFS=FS; print}'
# }
# 
# AWK=./gawk
# awktest > foo.gawk

BEGIN { FS = ":" }

# Have to reset OFS at end since not running separate invocations

FNR == 1 { $2 = "x"; OFS = FS; print ; OFS = " "}
FNR == 2 { $2 = "x"; print; OFS = FS; print ; OFS = " "}
FNR == 3 { $2 = "x"; print $1; OFS = FS; print ; OFS = " "}
FNR == 4 { $2 = "x"; print; $2 = $2; OFS = FS; print }
