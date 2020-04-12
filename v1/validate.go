package validate

import (
	"fmt"
	"reflect"
	"strings"
	"time"
)

import (
	"github.com/bww/epl"
	"github.com/bww/go-validate/stdlib"
)

var (
	ErrUnsupportedType = fmt.Errorf("Unsupported type (use a struct)")
)

type FieldError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

func FieldErrorf(f, m string, a ...interface{}) *FieldError {
	return &FieldError{f, fmt.Sprintf(m, a...)}
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

func (e Errors) Error() string {
	s := fmt.Sprintf("%d field errors", len(e))
	for _, x := range e {
		s += "\n  - " + x.Error()
	}
	return s
}

func keyPath(b, f string) string {
	if b != "" {
		return fmt.Sprintf("%s.%s", b, f)
	} else {
		return f
	}
}

type errorBuffer struct {
	E []error
}

func (e *errorBuffer) Add(v ...error) {
	e.E = append(e.E, v...)
}

type Validator struct {
	checkTag, nameTag string
}

func New() Validator {
	return NewWithTags("check", "json")
}

func NewWithTags(check, name string) Validator {
	return Validator{check, name}
}

func (v Validator) Validate(s interface{}) Errors {
	errs := &errorBuffer{}
	v.validate("", s, errs)
	return errs.E
}

func (v Validator) validate(p string, s interface{}, errs *errorBuffer) bool {
	z := reflect.Indirect(reflect.ValueOf(s))
	switch z.Kind() {
	case reflect.Invalid:
		return true
	case reflect.Struct:
		return v.validateStruct(p, z, errs)
	case reflect.Slice, reflect.Array:
		return v.validateSlice(p, z, errs)
	default:
		errs.Add(ErrUnsupportedType)
	}
	return false
}

func (v Validator) validateSlice(p string, z reflect.Value, errs *errorBuffer) bool {
	l := z.Len()
	valid := true
	for i := 0; i < l; i++ {
		if !v.validate(fmt.Sprintf("%s[%d]", p, i), z.Index(i).Interface(), errs) {
			valid = false
		}
	}
	return valid
}

func (v Validator) validateStruct(p string, z reflect.Value, errs *errorBuffer) bool {
	n := z.NumField()
	t := z.Type()

	valid := true
	for i := 0; i < n; i++ {
		field := t.Field(i)

		var path string
		if name := field.Tag.Get(v.nameTag); name != "" {
			path = keyPath(p, fieldName(name))
		} else {
			path = keyPath(p, field.Name)
		}

		src := field.Tag.Get(v.checkTag)
		if src != "" {
			expr, err := epl.Compile(src)
			if err != nil {
				errs.Add(FieldErrorf(path, "Could not compile expression: %v", err))
				valid = false
				continue
			}

			check := func(s interface{}) bool {
				return v.validate(path, s, errs)
			}

			date := func(y, m, d float64) time.Time {
				return time.Date(int(y), time.Month(m), int(d), 0, 0, 0, 0, time.UTC)
			}

			val := z.Field(i).Interface()
			cxt := map[string]interface{}{
				"self":  val,
				"sup":   z.Interface(),
				"len":   v.len,
				"now":   time.Now,
				"date":  date,
				"check": check,
				"str":   stdlib.Strings{},
			}

			res, err := expr.Exec(cxt)
			if err != nil {
				errs.Add(FieldErrorf(path, "Could not evaluate expression: %v", err))
				valid = false
				continue
			}

			if res != nil {
				switch c := res.(type) {
				case nil: // no error
				case error:
					errs.Add(c)
				case []error:
					errs.Add(c...)
				case bool:
					if !c {
						errs.Add(FieldErrorf(path, "Constraint not satisfied: %s", src))
						valid = false
					}
				default:
					errs.Add(FieldErrorf(path, "Invalid expression result: %T (expected %T) in %v", res, []error{}, res))
					valid = false
				}
			}
		}
	}

	return valid
}

func (v Validator) len(s interface{}) int {
	z := reflect.ValueOf(s)
	switch z.Kind() {
	case reflect.Invalid:
		return 0
	case reflect.Array, reflect.Chan, reflect.Map, reflect.Slice, reflect.String:
		return z.Len()
	default:
		panic(fmt.Errorf("Unsupported type: %T", s))
	}
}

func fieldName(t string) string {
	if x := strings.Index(t, ","); x > 0 {
		return t[:x]
	} else {
		return t
	}
}