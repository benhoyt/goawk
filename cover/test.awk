BEGIN {
  i=0 # TODO problem here with global i in generated END
  print "hello"
  callF()
  callF()
  callF()
  exit 0
}
function callF(){
  print "world"
}
END{}
