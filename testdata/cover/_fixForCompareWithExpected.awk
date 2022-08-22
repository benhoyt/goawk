# processes coverage report and turns absolute paths to just filenames for ease of comparison with expected

BEGIN {
  printf "" > OUT
}

NR > 1 {
  n = split($0,parts,"/")
  $0 = parts[n] # last
}

{
  print $0 >> OUT
}