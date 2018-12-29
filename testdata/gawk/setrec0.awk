function reassign(x, y) {
   $0 = x
   print y
}

{
   reassign("larry", $1)
}
