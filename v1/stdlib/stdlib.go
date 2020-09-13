package stdlib

import (
	"reflect"
)

func Indirect(v interface{}) interface{} {
	return reflect.Indirect(reflect.ValueOf(v)).Interface()
}
