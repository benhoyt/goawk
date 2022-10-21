BEGIN {
  __COVER["1"]++
  print "hello"
  callF()
  callF()
  callF()
  exit 0
}

BEGIN {
  __COVER["3"]++
  if (1) {
    __COVER["2"]++
    print "hello"
    print "world"
  }
  __COVER["5"]++
  print "end"
  for (i = 0; i < 7; i++) {
    __COVER["4"]++
    print i
  }
}

END {
  __COVER["6"]++
  print "END"
}

function callF() {
  __COVER["7"]++
  print "world"
}