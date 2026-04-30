package errors

import "fmt"

// Wrap annotates a sentinel with context. The sentinel remains accessible via errors.Is.
//
//	if u == nil {
//	    return errors.Wrap(errors.ErrNotFound, "user", "id", id)
//	}
//
// Output: "user (id=...): not found"
func Wrap(sentinel error, msg string, kvs ...any) error {
	return wrap(sentinel, formatKV(msg, kvs...))
}

func wrap(sentinel error, msg string) error {
	if msg == "" {
		return sentinel
	}
	return fmt.Errorf("%s: %w", msg, sentinel)
}

func formatKV(msg string, kvs ...any) string {
	if len(kvs) == 0 {
		return msg
	}
	out := msg
	if len(kvs)%2 == 0 {
		out += " ("
		for i := 0; i < len(kvs); i += 2 {
			if i > 0 {
				out += ", "
			}
			out += fmt.Sprintf("%v=%v", kvs[i], kvs[i+1])
		}
		out += ")"
	}
	return out
}
