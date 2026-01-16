package eloquent

type ValidationError struct {
	Errors map[string]string
}

func (e *ValidationError) Error() string {
	return "validation failed"
}

type NotFoundError struct {
	Table string
	PK    any
}

func (e *NotFoundError) Error() string {
	return "not found"
}
