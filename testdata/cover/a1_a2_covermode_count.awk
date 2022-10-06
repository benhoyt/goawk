BEGIN {
  __COVER[1]++
  print "hello"
  callF()
  callF()
  callF()
  exit 0
}

BEGIN {
  if (1) {
    __COVER[2]++
    print "hello"
    print "world"
  }
  __COVER[4]++
  print "end"
  for (i = 0; i < 7; i++) {
    __COVER[3]++
    print i
  }
}

END {
  __COVER[5]++
  print "END"
}

function callF() {
  __COVER[6]++
  print "world"
}
