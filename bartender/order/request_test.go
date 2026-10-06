package order

import (
	"testing"

	"go.viam.com/test"
)

func TestDecodeRequestHappy(t *testing.T) {
	req, err := DecodeRequest(map[string]any{"drink": "negroni"})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, req.Drink, test.ShouldEqual, "negroni")
}

func TestDecodeRequestNonObject(t *testing.T) {
	_, err := DecodeRequest("negroni")
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "must be an object")
}

func TestDecodeRequestMissingDrink(t *testing.T) {
	_, err := DecodeRequest(map[string]any{})
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "required")
}

func TestDecodeRequestWrongType(t *testing.T) {
	_, err := DecodeRequest(map[string]any{"drink": 42})
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "drink")
}
