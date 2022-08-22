BEGIN {
  print "hello"
  callF()
  callF()
  callF()
  exit 0
}
function callF(){
  print "world"
}
END{ print "END" }
