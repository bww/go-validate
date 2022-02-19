package validate

import (
	"fmt"
)

type FieldError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
	Cause   error  `json:"-"`
}

func newFieldError(f string, err error) *FieldError {
	return &FieldError{f, err.Error(), err}
}

func FieldErrorf(f, m string, a ...interface{}) *FieldError {
	return &FieldError{f, fmt.Sprintf(m, a...), nil}
}

func (e FieldError) Unwrap() error {
	return e.Cause
}

func (e FieldError) Error() string {
	return fmt.Sprintf("%v: %v", e.Field, e.Message)
}

type Errors []error

func (e Errors) Fields() []string {
	fields := make([]string, 0)
	for _, v := range e {
		switch c := v.(type) {
		case FieldError:
			fields = append(fields, c.Field)
		case *FieldError:
			fields = append(fields, c.Field)
		}
	}
	return fields
}

func (e Errors) Messages() []string {
	msgs := make([]string, 0)
	for _, v := range e {
		switch c := v.(type) {
		case FieldError:
			msgs = append(msgs, c.Message)
		case *FieldError:
			msgs = append(msgs, c.Message)
		}
	}
	return msgs
}

func (e Errors) Error() string {
	s := fmt.Sprintf("%d field errors", len(e))
	for _, x := range e {
		s += "\n  - " + x.Error()
	}
	return s
}
