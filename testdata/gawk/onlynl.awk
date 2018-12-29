BEGIN { RS = "" }
{ print "got", $0 }
