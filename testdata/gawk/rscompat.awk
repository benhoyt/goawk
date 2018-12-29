BEGIN { RS = "bar" }
{ print $1, $2 }
