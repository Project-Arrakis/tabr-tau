package save

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
)

// hashRows is a hash of the rows a query returned, in the order returned: every column of every row with its type, so 5 and 5.0 and '5' differ.
func hashRows(rows []Row) string { return hashRowsDigest(rows, nil) }

// hashRowsDigest is hashRows that also takes the digest of each blob longer than the 24 bytes the rows show (digs[i][column]): the text of such a blob stops at the 24th byte,
// and a change after it must change the hash. A blob of 24 bytes or fewer hashes as it always did, so a journal kept by an earlier version still matches.
func hashRowsDigest(rows []Row, digs []map[string]string) string {
	h := sha256.New()
	for ri, r := range rows {
		keys := make([]string, 0, len(r))
		for k := range r {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			switch v := r[k].(type) {
			case []byte:
				fmt.Fprintf(h, "%q=x:%x;", k, v)
			default:
				if ri < len(digs) && digs[ri][k] != "" {
					fmt.Fprintf(h, "%q=%T:%v#%s;", k, v, v, digs[ri][k])
				} else {
					fmt.Fprintf(h, "%q=%T:%v;", k, v, v)
				}
			}
		}
		h.Write([]byte{'\n'})
	}
	return hex.EncodeToString(h.Sum(nil))
}

// readable says whether a statement is a single SELECT (or a WITH leading into one that does not write): the only kind of query the journal runs again.
func readable(q string) bool {
	s := strings.TrimRight(strings.TrimSpace(q), "; \t\r\n")
	if s == "" || !oneStatement(q) {
		return false
	}
	w := strings.ToLower(s)
	if i := strings.IndexFunc(w, func(r rune) bool { return r == ' ' || r == '\t' || r == '\n' || r == '\r' || r == '(' }); i > 0 {
		w = w[:i]
	}
	switch w {
	case "select":
		return true
	case "with":
		for _, bad := range []string{"insert", "update", "delete", "replace"} {
			if strings.Contains(strings.ToLower(s), bad) {
				return false
			}
		}
		return true
	}
	return false
}
