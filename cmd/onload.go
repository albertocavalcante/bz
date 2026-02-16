package cmd

//nolint:unparam // package-level command wiring requires a value expression.
func onLoad(fn func()) struct{} {
	fn()
	return struct{}{}
}
