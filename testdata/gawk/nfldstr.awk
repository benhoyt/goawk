$1 == 0 {
	print "bug"
}
{
	$0 = "0" 
	if (!$0)
		print "another bug"
	$0 = a = "0" 
	if (!$0)
		print "yet another bug"
	if ($1)
		print "a buggie"
}
