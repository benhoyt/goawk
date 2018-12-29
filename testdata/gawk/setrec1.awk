function reassign(x, y) {
   $0 = x
   print y
}

BEGIN {
   $0 = substr("geronimo", 5, 3)
   reassign(" 52", $1)
}
