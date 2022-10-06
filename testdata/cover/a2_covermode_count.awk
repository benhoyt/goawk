BEGIN {
  if (1) {
    __COVER[1]++
    print "hello"
    print "world"
  }
  __COVER[3]++
  print "end"
  for (i = 0; i < 7; i++) {
    __COVER[2]++
    print i
  }
}
