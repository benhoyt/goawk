# from Pat Rankin, rankin@eql.caltech.edu, now rankin@pactechdata.com

BEGIN { dummy(1); legit(); exit }

function dummy(arg)
{
	return arg
}

function legit(         scratch)
{
	split("1 2 3", scratch)
	return ""
}
