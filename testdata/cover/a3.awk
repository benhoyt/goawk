BEGIN {
  if (1) {
    for (i=0; i<10; i++) {
      print i
      while (1) {
        do {
          for (j in A) {
            print j
            if (2) {
              print 2
              {
                if (3) print 3
              }
            } else {
              continue
            }
          }
        } while(1)
      }
    }
  }
}