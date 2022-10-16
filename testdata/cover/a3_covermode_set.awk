BEGIN {
  __COVER["10"] = 1
  if (1) {
    __COVER["9"] = 1
    for (i = 0; i < 10; i++) {
      __COVER["8"] = 1
      print i
      while (1) {
        __COVER["7"] = 1
        do {
          __COVER["6"] = 1
          for (j in A) {
            __COVER["5"] = 1
            print j
            if (2) {
              __COVER["3"] = 1
              print 2
              {
                __COVER["2"] = 1
                if (3) {
                  __COVER["1"] = 1
                  print 3
                }
              }
            } else {
              __COVER["4"] = 1
              continue
            }
          }
        } while (1)
      }
    }
  }
}