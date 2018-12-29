# This should exit 0, even though child exits 1
BEGIN { "exit 1" | getline junk ; exit 12 }
END { exit 0 }
