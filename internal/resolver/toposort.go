// Topological sorting

package resolver

/*
This algorithm is taken from:
https://en.wikipedia.org/wiki/Topological_sorting#Depth-first_search

L ← Empty list that will contain the sorted nodes
while exists nodes without a permanent mark do
    select an unmarked node n
    visit(n)

function visit(node n)
    if n has a permanent mark then
        return
    if n has a temporary mark then
        stop   (not a DAG)

    mark n with a temporary mark

    for each node m with an edge from n to m do
        visit(m)

    remove temporary mark from n
    mark n with a permanent mark
    add n to head of L
*/

// Perform a topological sort on the given graph.
func topoSort(graph map[string]map[string]struct{}) []string {
	if len(graph) == 0 {
		return nil
	}

	unmarked := make(map[string]struct{})
	for node := range graph {
		unmarked[node] = struct{}{}
	}
	permMarks := make(map[string]struct{})
	tempMarks := make(map[string]struct{})
	var sorted []string

	var visit func(string)
	visit = func(n string) {
		if _, ok := permMarks[n]; ok {
			return
		}
		if _, ok := tempMarks[n]; ok {
			return
		}
		tempMarks[n] = struct{}{}
		for m := range graph[n] {
			visit(m)
		}
		delete(tempMarks, n)
		permMarks[n] = struct{}{}
		delete(unmarked, n)
		sorted = append(sorted, n)
		return
	}

	for len(unmarked) > 0 {
		var n string
		for n = range unmarked {
			break
		}
		visit(n)
	}

	return sorted
}
