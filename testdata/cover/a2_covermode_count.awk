BEGIN {
  __COVER["2"]++
  if (1) {
    __COVER["1"]++
    print "hello"
    print "world"
  }
  __COVER["4"]++
  print "end"
  for (i = 0; i < 7; i++) {
    __COVER["3"]++
    print i
  }
}