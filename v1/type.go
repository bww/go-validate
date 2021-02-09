package validate

import (
	"reflect"
	"strings"
)

type typeKey struct {
	Type   reflect.Type
	Config Config
}

func newTypeKey(t reflect.Type, v Validator) typeKey {
	return typeKey{
		Type: t,
		Config: Config{
			CheckTag: v.checkTag,
			ErrorTag: v.errTag,
			FieldTag: v.nameTag,
		},
	}
}

type validatedField struct {
	Name    string
	Message string
	Expr    string
	Index   int
	Field   reflect.StructField
}

type validatedType struct {
	Type   reflect.Type
	Fields []validatedField
}

func newType(t reflect.Type, v Validator) *validatedType {
	n := t.NumField()
	f := make([]validatedField, 0, n)

	for i := 0; i < n; i++ {
		x := t.Field(i)

		var name string
		if v := x.Tag.Get(v.nameTag); v != "" {
			name = fieldName(v)
		} else if !x.Anonymous { // embedded fields don't get an inferred name
			name = x.Name
		}

		msg := strings.TrimSpace(x.Tag.Get(v.errTag))
		src := strings.TrimSpace(getTag(x.Tag, v.checkTag))
		if src == "-" {
			continue
		} else if src == "" && !x.Anonymous {
			continue
		}

		f = append(f, validatedField{
			Name:    name,
			Message: msg,
			Expr:    src,
			Index:   i,
			Field:   x,
		})
	}

	return &validatedType{
		Type:   t,
		Fields: f,
	}
}

func fieldName(t string) string {
	if x := strings.Index(t, ","); x > 0 {
		return t[:x]
	} else {
		return t
	}
}
