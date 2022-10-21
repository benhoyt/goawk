BEGIN {
  __COVER["1"] = 1
  print "hello"
  callF()
  callF()
  callF()
  exit 0
}

END {
  __COVER["2"] = 1
  print "END"
}

function callF() {
  __COVER["3"] = 1
  print "world"
}