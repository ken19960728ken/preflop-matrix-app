package engine

// PyRepr formats s the way Python's repr() does (quote choice and escapes), for messages built outside the engine.
func PyRepr(s string) string { return pyRepr(s) }
