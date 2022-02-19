package validate

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFieldErrors(t *testing.T) {
	someErr := errors.New("Some error")
	ferr := newFieldError("field", someErr)
	assert.Equal(t, true, errors.Is(ferr, someErr))
}
