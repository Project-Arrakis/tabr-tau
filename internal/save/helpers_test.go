package save

// Test helpers: production code has exactly one write path (Mutate); tests that only need "run this SQL" use these.

func (s *Save) Exec(q string, args ...any) (int64, error) {
	return s.Mutate("test edit", func(m *Mut) error { _, err := m.Exec(q, args...); return err })
}

func (s *Save) ExecScript(script string) (int64, error) { return s.Exec(script) }
