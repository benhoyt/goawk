BEGIN{ X() }

function X(	abc)
{
	abc = "stamp out "
	print abc abc()
}

function abc() { return "dark corners" }
