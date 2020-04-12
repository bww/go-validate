package validate

import (
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
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

func TestValidate(t *testing.T) {
	v := New()

	checkValid(t, v, testA{}, []string{"a_1"})
	checkValid(t, v, testA{"A"}, nil)

	checkValid(t, v, testB{}, []string{"b_1"})
	checkValid(t, v, testB{&testA{}}, []string{"b_1.a_1", "b_1"})
	checkValid(t, v, testB{&testA{"A"}}, nil)

	checkValid(t, v, testC{0, 0, 0, 0, 1}, []string{"c_1", "c_4", "c_5"})
	checkValid(t, v, testC{1, -1, 0, 1, 1}, nil)

	checkValid(t, v, testD{}, []string{"d_1"})
	checkValid(t, v, testD{true}, nil)

	checkValid(t, v, testE{}, []string{"e_1"})
	checkValid(t, v, testE{[]int{1, 2}}, nil)

	checkValid(t, v, testF{}, []string{"f_1"})
	checkValid(t, v, testF{[]testA{{}}}, []string{"f_1[0].a_1", "f_1"})
	checkValid(t, v, testF{[]testA{{"A"}}}, nil)

	checkValid(t, v, testG{}, []string{"g_1", "g_2"})
	checkValid(t, v, testG{time.Now(), time.Time{}}, []string{"g_2"})
	checkValid(t, v, testG{time.Now(), time.Now().Add(time.Minute)}, nil)

	checkValid(t, v, testH{}, []string{"h_1"})
	checkValid(t, v, testH{&testB{}}, []string{"h_1.b_1", "h_1"})
	checkValid(t, v, testH{&testB{&testA{}}}, []string{"h_1.b_1.a_1", "h_1.b_1", "h_1"})
	checkValid(t, v, testH{&testB{&testA{"A"}}}, nil)

	checkValid(t, v, testI{}, []string{"i_2", "i_3", "i_4"})
	checkValid(t, v, testI{"", "Abc", "123Abc", "987"}, nil)

}

func checkValid(t *testing.T, v Validator, e interface{}, expect []string) {
	actual := v.Validate(e)
	if len(expect) == 0 {
		assert.Len(t, actual, 0)
	} else {
		assert.Equal(t, expect, actual.Fields(), actual.Error())
	}
}