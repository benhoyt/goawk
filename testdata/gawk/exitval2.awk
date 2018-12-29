BEGIN { print "foo" | "read x ; echo $x ; exit 12" }
# this should still exit 0, as pointed out by kenny mccormack in
# comp.lang.awk on 2 feb 2005
