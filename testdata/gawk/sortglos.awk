BEGIN {
	pr="y";
	npre=0;
	po="n";
	npos=0;
	}

pr=="y"      { npre++; pre[npre]=$0; }
$1=="@table" && $2=="@asis" { pr="n";nite++; next; }

po=="y"      { npos++; pos[npos]=$0; }
$1=="@end" && $2=="table"   {
	po="y";
	npos++;
	pos[npos]=$0;
	# last item...
	vec[nite]=nlin;
}

	{ nite++; }

END {
	for ( i=1; i<=npre; i++ ) { print pre[i]; }
	if ( srt=="y" ) {
		n=asorti(entr,ital);
		##print "n=",n;
		for ( i=1; i<=n; i++ ) {
			#printf("=========> %3.3d %s\n",i,ital[i]);
			# ital[i] is the sorted key;
			j=entr[ital[i]];
			# j is the original item number
			for ( k=1; k<=vec[j]; k++ ) {
				print dat[j,k];
			}
		}
	}
	if ( srt=="n" ) {
		for ( i=1; i<=nite; i++ ) {
			printf("=========> %3.3d %2.2d %s\n",i,vec[i],titl[i]);
			for ( j=1; j<=vec[i]; j++ ) {
				print dat[i,j];
			}
		}
		print "========================= END";
	}
	for ( i=1; i<=npos; i++ ) { print pos[i]; }
	print "@c npre=" npre;
	print "@c nite=" nite;
	print "@c npos=" npos;
}

