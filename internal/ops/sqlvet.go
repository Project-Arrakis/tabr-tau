package ops

import (
	"errors"
	"fmt"
	"strings"
)

// The SQL console vets statement text before it reaches SQLite. This is the first of two layers: the read console
// also runs on a connection that cannot write at all (save.ReadOnlyTable), so a mistake here cannot change data.

// sqlStatement is one statement with comments removed: its words at parenthesis depth 0 and anywhere.
type sqlStatement struct {
	top []string // lower-cased words outside parentheses
	all []string // lower-cased words at any depth (string and quoted-identifier contents excluded)
}

// splitSQL tokenises script, honouring '...' strings, "..." `...` [...] identifiers and both comment styles.
// It returns the non-empty statements and refuses unterminated strings, identifiers and comments.
func splitSQL(script string) ([]sqlStatement, error) {
	var stmts []sqlStatement
	var cur sqlStatement
	depth := 0
	flush := func() {
		if len(cur.all) > 0 {
			stmts = append(stmts, cur)
		}
		cur = sqlStatement{}
		depth = 0
	}
	n := len(script)
	for i := 0; i < n; {
		c := script[i]
		switch {
		case c == '-' && i+1 < n && script[i+1] == '-':
			for i < n && script[i] != '\n' {
				i++
			}
		case c == '/' && i+1 < n && script[i+1] == '*':
			end := strings.Index(script[i+2:], "*/")
			if end < 0 {
				return nil, errors.New("unterminated comment")
			}
			i += 2 + end + 2
		case c == '\'' || c == '"' || c == '`' || c == '[':
			closer := c
			if c == '[' {
				closer = ']'
			}
			j := i + 1
			for {
				if j >= n {
					return nil, errors.New("unterminated quote")
				}
				if script[j] == closer {
					if closer != ']' && j+1 < n && script[j+1] == closer { // doubled quote escapes itself
						j += 2
						continue
					}
					break
				}
				j++
			}
			// Quoted identifiers AND string literals are recorded by content (SQLite accepts a string literal where a
			// table name is expected), marked with \x00 so they can never be mistaken for a keyword.
			cur.all = append(cur.all, "\x00"+strings.ToLower(script[i+1:j]))
			if depth == 0 {
				cur.top = append(cur.top, "\x00")
			}
			i = j + 1
		case c == ';':
			flush()
			i++
		case c == '(':
			depth++
			i++
		case c == ')':
			if depth > 0 {
				depth--
			}
			i++
		case isWordByte(c):
			j := i
			for j < n && isWordByte(script[j]) {
				j++
			}
			w := strings.ToLower(script[i:j])
			cur.all = append(cur.all, w)
			if depth == 0 {
				cur.top = append(cur.top, w)
			}
			i = j
		default:
			i++
		}
	}
	flush()
	return stmts, nil
}

func isWordByte(c byte) bool {
	return c == '_' || c >= '0' && c <= '9' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= 0x80
}

// mainVerb is the statement's real verb: its first word, or for WITH the first select/insert/update/delete/replace
// outside parentheses (so "with x as (select 1) insert ..." is an insert, not a select).
func (s sqlStatement) mainVerb() string {
	if len(s.top) == 0 {
		return ""
	}
	if s.top[0] != "with" {
		return s.top[0]
	}
	for _, w := range s.top[1:] {
		switch w {
		case "select", "insert", "update", "delete", "replace":
			return w
		}
	}
	return "with"
}

// introspection lists the pragma table-valued functions the read console may call: they only describe the schema.
var introspection = map[string]bool{
	"pragma_table_info": true, "pragma_table_xinfo": true, "pragma_table_list": true, "pragma_index_list": true,
	"pragma_index_info": true, "pragma_index_xinfo": true, "pragma_foreign_key_list": true,
}

// forbidden refuses, in either console, anything that could reach SQLite internals: load_extension, pragma_*
// table-valued functions (they accept arguments and can change settings; only the schema-describing ones above
// are allowed, and only when reading), and sqlite_* objects other than
// the read-only catalogue views sqlite_master / sqlite_schema. Quoted names and string literals are checked by
// content too. A literal containing a % wildcard (a LIKE pattern such as 'sqlite_%') is not a table name.
func (s sqlStatement) forbidden(reading bool) error {
	for _, w := range s.all {
		name := strings.TrimPrefix(w, "\x00")
		quoted := name != w
		switch {
		case name == "load_extension":
			return errors.New("load_extension is not allowed")
		case strings.HasPrefix(name, "pragma_"):
			if !reading || quoted || !introspection[name] {
				return errors.New("pragma functions are not allowed")
			}
		case strings.HasPrefix(name, "sqlite_"):
			if name == "sqlite_master" || name == "sqlite_schema" || (quoted && strings.Contains(name, "%")) {
				continue
			}
			return fmt.Errorf("%s is not available", name)
		}
	}
	return nil
}

// vetRead accepts exactly one SELECT (optionally with a WITH prefix) and nothing else.
func vetRead(q string) error {
	stmts, err := splitSQL(q)
	if err != nil {
		return err
	}
	if len(stmts) == 0 {
		return errors.New("SQL is required")
	}
	if len(stmts) > 1 {
		return errors.New("only one statement can be run at a time")
	}
	if err := stmts[0].forbidden(true); err != nil {
		return err
	}
	if v := stmts[0].mainVerb(); v != "select" {
		return fmt.Errorf("only SELECT queries are allowed here, not %q (use the write console for edits)", v)
	}
	return nil
}

// vetWrite accepts one or more INSERT / UPDATE / DELETE / REPLACE statements and nothing else: no PRAGMA, ATTACH,
// transaction control, DDL, VACUUM, or reads.
func vetWrite(q string) error {
	stmts, err := splitSQL(q)
	if err != nil {
		return err
	}
	if len(stmts) == 0 {
		return errors.New("SQL is required")
	}
	for _, st := range stmts {
		if err := st.forbidden(false); err != nil {
			return err
		}
		for _, w := range st.all {
			switch strings.TrimPrefix(w, "\x00") {
			case "sqlite_master", "sqlite_schema":
				return errors.New("the schema catalogue cannot be edited")
			case "applied_patches":
				return errors.New("applied_patches records the game's version history and cannot be edited here")
			case "accounts":
				return errors.New("accounts identifies the character to the game and cannot be edited here")
			}
		}
		switch v := st.mainVerb(); v {
		case "insert", "update", "delete", "replace":
		default:
			return fmt.Errorf("only INSERT, UPDATE, DELETE and REPLACE are allowed here, not %q", v)
		}
	}
	return nil
}
