//go:build !windows

package save

// driveProblem has nothing to say off Windows: the guard itself only runs there.
func driveProblem(string) string { return "" }
