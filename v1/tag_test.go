package validate

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTag(t *testing.T) {
	type S struct {
		F string `species:"gopher" color,couleur:"blue"`
	}

	f := reflect.TypeOf(S{}).Field(0)
	var v string
	var ok bool

	v, ok = findTag(f.Tag, "color")
	assert.Equal(t, true, ok)
	assert.Equal(t, "blue", v)

	v, ok = findTag(f.Tag, "couleur")
	assert.Equal(t, true, ok)
	assert.Equal(t, "blue", v)

	v, ok = findTag(f.Tag, "nah")
	assert.Equal(t, false, ok)
	assert.Equal(t, "", v)
}
