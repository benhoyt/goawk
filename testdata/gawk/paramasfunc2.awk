BEGIN{ X() }

function abc() { return "dark corners" }

function X(	abc)
{
	abc = "stamp out "
	print abc abc()
}

