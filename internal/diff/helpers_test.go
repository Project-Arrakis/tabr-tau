package diff_test

import "encoding/json"

// jsonMarshal round-trips v through JSON so tests can walk it generically.
func jsonMarshal(v any) (any, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	var out any
	err = json.Unmarshal(b, &out)
	return out, err
}
