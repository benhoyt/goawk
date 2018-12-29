# from Stepan Kasal, 9 July 2003
function f()
{
	delete f
}

BEGIN { f() }
