package uiexec

// FindNode is a convenience wrapper matching the spec-style API.
// For repeated queries, prefer constructing a Finder once via NewFinder(nodes).
func FindNode(nodes []Node, query Query) *Node {
	return NewFinder(nodes).FindNode(query)
}

// OpportunisticExecution is a small helper that exposes the spec-style
// TryDirectAction(target, intent) surface while keeping dependencies explicit.
type OpportunisticExecution struct {
	Finder *Finder
	Exec   func(Action) error
}

func (o OpportunisticExecution) TryDirectAction(target string, intent string) bool {
	return TryDirectAction(o.Finder, o.Exec, target, intent)
}
