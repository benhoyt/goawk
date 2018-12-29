{
   if (substr($1,1,1) == substr($0,1,1))
      print "substr matches"
   sub(/foo/,"bar")
   print nr++
}
