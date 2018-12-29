BEGIN { FS = "\0" ; RS = "" }
{ print $2 }
