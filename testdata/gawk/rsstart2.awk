BEGIN { RS = "^Ax*\n" }
END { print NR }
