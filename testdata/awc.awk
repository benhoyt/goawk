BEGIN {
  for (i = 1; i < ARGC; i++) {
    if (ARGV[i] == "--") {
      delete ARGV[i++]
      break
    }
    else if (ARGV[i] !~ /^-./) break
    else if (ARGV[i] == "-c") c = 1
    else if (ARGV[i] == "-w") w = 1
    else if (ARGV[i] == "-l") l = 1
    else printf "awc: unknown option: %s\n", ARGV[i] >"/dev/stderr"
    delete ARGV[i]
  }
  if (!c && !w && !l) c = w = l = 1
}
{ cs += length(); ws += NF; ls++ }
END { printf "%s%s%s\n", l?ls" ":"", w?ws" ":"", c?cs" ":"" }
