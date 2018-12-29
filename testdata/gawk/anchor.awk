BEGIN { RS = "" }

{
	if (/^A/)
		print "ok"
	else
		print "not ok"

	if (/B$/)
		print "not ok"
	else
		print "ok"

	if (/^C/)
		print "not ok"
	else
		print "ok"

	if (/D$/)
		print "not ok"
	else
		print "ok"

	if (/^E/)
		print "not ok"
	else
		print "ok"

	if (/F$/)
		print "ok"
	else
		print "not ok"
}
