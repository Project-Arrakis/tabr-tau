//go:build !windows

package save

// processImages is not used off Windows (the game check returns at once there); it exists so the package builds.
func processImages() ([]string, error) { return nil, nil }
