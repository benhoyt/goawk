BEGIN {
    A[0] = "&"
    for(i=1;i<=11;i++) {
	A[i] = "\\" A[i-1]
    }
## A[] holds  & \& \\& \\\& \\\\& ...
}

{
    for(i=0; i <= 11 ; i++) {
        x = $0 
        sub(/B/, A[i], x)
	print i, x
    }
}
