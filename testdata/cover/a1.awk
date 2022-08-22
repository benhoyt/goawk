BEGIN {
  print "hello"
  callF()
  callF()
  callF()
  exit 0 # this will call END
}
function callF(){
  print "world"
}
END{ print "END" }
