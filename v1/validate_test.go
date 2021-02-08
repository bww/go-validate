package validate

import (
	"fmt"
	"testing"
	"time"

	"github.com/hashicorp/golang-lru"
	"github.com/stretchr/testify/assert"
)

type testA struct {
	F1 string `json:"a_1" check:"len(self) > 0"`
}

type testB struct {
	F1 *testA `json:"b_1" check:"self != nil && check(self)"`
}

type testC struct {
	F1 int `json:"c_1" check:"self >  0"`
	F2 int `json:"c_2" check:"self <= 0"`
	F3 int `json:"c_3" check:"self == 0"`
	F4 int `json:"c_4" check:"self != 0"`
	F5 int `json:"c_5" check:"self == sup.F1"`
}

type testD struct {
	F1 bool `json:"d_1" check:"self"`
}

type testE struct {
	F1 []int `json:"e_1" check:"len(self) > 0"`
}

type testF struct {
	F1 []testA `json:"f_1" check:"len(self) > 0 && check(self)"`
}

type testG struct {
	F1 time.Time `json:"g_1" check:"self.After(date(2018, 1, 1))"`
	F2 time.Time `json:"g_2" check:"self.After(now())"`
}

type testH struct {
	F1 *testB `json:"h_1" check:"self != nil && check(self)"`
}

type testI struct {
	F1 string `json:"i_1" check:"str.Alpha(self)"`
	F2 string `json:"i_2" check:"len(self) > 0 && str.Alpha(self)"`
	F3 string `json:"i_3" check:"len(self) > 0 && str.AlphaNumeric(self)"`
	F4 string `json:"i_4" check:"len(self) > 0 && str.Numeric(self)"`
}

type testJ struct {
	F1 string `json:"j_1" check:"len(self) > 0" invalid:"F1 must not be empty"`
	F2 int    `json:"j_2" check:"self > 0"  invalid:"F2 must not be zero, gotta be bigger"`
}

type fieldA string

func (f fieldA) Validate() error {
	l := len(f)
	if l < 1 || l > 10 {
		return fmt.Errorf("Must be between 1-10 bytes")
	}
	return nil
}

type testK struct {
	F1 fieldA `json:"k_1" check:"check "` // must be 1-10 bytes, inclusive
	F2 string `json:"k_2" check:"len(self) == 0"`
}

type testL struct {
	F1 string `json:"l_1" check:"len(self) == 0 || str.Match(\"#[0-9a-f]{6}\", self)" invalid:"Pattern doesn't match"`
}

type testM struct {
	F1 string `json:"m_1" check:"len(self) == 3" invalid:"Wrong length"`
}

func (s testM) Validate() error { return nil } //v1

type testN struct {
	F1 string `json:"n_1" check:"len(self) == 3" invalid:"Wrong length"`
}

func (s testN) Validate(v Validator) (error, bool) { return nil, true } //v2

type testO struct {
	F1 string `json:"o_1" check:"len(self) == 3" invalid:"Wrong length"`
}

func (s testO) Validate(v Validator) (error, bool) { // v2
	return FieldErrorf("syn", "This is the problem"), true
}

type testP struct {
	testA
}
type testQ struct {
	testA `check:"-"`
}

func TestValidate(t *testing.T) {
	v := New()

	checkValid(t, v, testA{}, []string{"a_1"}, nil)
	checkValid(t, v, testA{"A"}, nil, nil)

	checkValid(t, v, testB{}, []string{"b_1"}, nil)
	checkValid(t, v, testB{&testA{}}, []string{"b_1.a_1", "b_1"}, nil)
	checkValid(t, v, testB{&testA{"A"}}, nil, nil)

	checkValid(t, v, testC{0, 0, 0, 0, 1}, []string{"c_1", "c_4", "c_5"}, nil)
	checkValid(t, v, testC{1, -1, 0, 1, 1}, nil, nil)

	checkValid(t, v, testD{}, []string{"d_1"}, nil)
	checkValid(t, v, testD{true}, nil, nil)

	checkValid(t, v, testE{}, []string{"e_1"}, nil)
	checkValid(t, v, testE{[]int{1, 2}}, nil, nil)

	checkValid(t, v, testF{}, []string{"f_1"}, nil)
	checkValid(t, v, testF{[]testA{{}}}, []string{"f_1[0].a_1", "f_1"}, nil)
	checkValid(t, v, testF{[]testA{{"A"}}}, nil, nil)

	checkValid(t, v, testG{}, []string{"g_1", "g_2"}, nil)
	checkValid(t, v, testG{time.Now(), time.Time{}}, []string{"g_2"}, nil)
	checkValid(t, v, testG{time.Now(), time.Now().Add(time.Minute)}, nil, nil)

	checkValid(t, v, testH{}, []string{"h_1"}, nil)
	checkValid(t, v, testH{&testB{}}, []string{"h_1.b_1", "h_1"}, nil)
	checkValid(t, v, testH{&testB{&testA{}}}, []string{"h_1.b_1.a_1", "h_1.b_1", "h_1"}, nil)
	checkValid(t, v, testH{&testB{&testA{"A"}}}, nil, nil)

	checkValid(t, v, testI{}, []string{"i_2", "i_3", "i_4"}, nil)
	checkValid(t, v, testI{"", "Abc", "123Abc", "987"}, nil, nil)

	checkValid(t, v, testJ{}, []string{"j_1", "j_2"}, []string{"F1 must not be empty", "F2 must not be zero, gotta be bigger"})

	checkValid(t, v, testK{}, []string{"k_1"}, []string{"Must be between 1-10 bytes"})
	checkValid(t, v, testK{F1: "This is too long. Way too long. Fix it."}, []string{"k_1"}, []string{"Must be between 1-10 bytes"})
	checkValid(t, v, testK{F1: "A"}, nil, nil)

	checkValid(t, v, testL{}, nil, nil)
	checkValid(t, v, testL{F1: "#ff0033"}, nil, nil)
	checkValid(t, v, testL{F1: "#ff003"}, []string{"l_1"}, []string{"Pattern doesn't match"})
	checkValid(t, v, testL{F1: "_ff0033"}, []string{"l_1"}, []string{"Pattern doesn't match"})

	// v1 doesn't support fields checks
	checkValid(t, v, testM{F1: "111"}, nil, nil)
	checkValid(t, v, testM{F1: "1"}, nil, nil)
	// v2 does
	checkValid(t, v, testN{F1: "111"}, nil, nil)
	checkValid(t, v, testN{F1: "1"}, []string{"n_1"}, []string{"Wrong length"})
	// v2 supports both
	checkValid(t, v, testO{F1: "111"}, []string{"syn"}, []string{"This is the problem"})
	checkValid(t, v, testO{F1: "1"}, []string{"syn", "o_1"}, []string{"This is the problem", "Wrong length"})

	checkValid(t, v, testP{}, []string{"a_1"}, nil)
	checkValid(t, v, testP{testA{"Hello"}}, nil, nil)
	checkValid(t, v, testQ{}, nil, nil)
	checkValid(t, v, testQ{testA{"Hello"}}, nil, nil)
}

func checkValid(t *testing.T, v Validator, e interface{}, expect []string, errmsg []string) {
	actual := v.Validate(e)
	if len(expect) == 0 {
		assert.Len(t, actual, 0)
	} else if assert.Equal(t, expect, actual.Fields(), actual.Error()) {
		fmt.Println("*** ", actual.Messages())
		if errmsg != nil {
			assert.Equal(t, errmsg, actual.Messages(), actual.Error())
		}
	}
}

type modeA struct {
	F1 string `json:"a_1" create:"len(self) > 0" invalid:"Wrong length"`
	F2 string `json:"a_2" create,update:"len(self) > 0" invalid:"Wrong length"`
	F3 string `json:"a_3"`
}

func TestMode(t *testing.T) {
	var errs Errors
	v1 := modeA{}
	errs = New(Mode("create")).Validate(v1)
	assert.Equal(t, []string{"a_1", "a_2"}, errs.Fields())
	errs = New(Mode("update")).Validate(v1)
	assert.Equal(t, []string{"a_2"}, errs.Fields())
	errs = New(Mode("never_heard_of_it")).Validate(v1)
	assert.Equal(t, []string{}, errs.Fields())
	errs = New(Mode("create,update")).Validate(v1)
	assert.Equal(t, []string{}, errs.Fields())
}

func BenchmarkValidateWithCache(b *testing.B) {
	cache, _ = lru.New(256)
	v := New()
	for i := 0; i < b.N; i++ {
		v.Validate(testA{})
		v.Validate(testA{"A"})

		v.Validate(testB{})
		v.Validate(testB{&testA{}})
		v.Validate(testB{&testA{"A"}})

		v.Validate(testC{0, 0, 0, 0, 1})
		v.Validate(testC{1, -1, 0, 1, 1})
	}
}

func BenchmarkValidateWithoutCache(b *testing.B) {
	cache = nil
	v := New()
	for i := 0; i < b.N; i++ {
		v.Validate(testA{})
		v.Validate(testA{"A"})

		v.Validate(testB{})
		v.Validate(testB{&testA{}})
		v.Validate(testB{&testA{"A"}})

		v.Validate(testC{0, 0, 0, 0, 1})
		v.Validate(testC{1, -1, 0, 1, 1})
	}
}
