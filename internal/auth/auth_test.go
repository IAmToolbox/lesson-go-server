package auth

import (
	"testing"
	"time"
	"reflect"

	"github.com/google/uuid"
)

func TestMakeJWT(t *testing.T) {
	token, err := MakeJWT(uuid.New(), "the seecreeeeet", 5 * time.Second)
	if reflect.TypeOf(token).Kind() != reflect.String {
		t.Errorf(`Token: %v Error: %v`, token, err)
	}
}

func TestParseToken(t *testing.T) {
	id := uuid.New()
	secret:= "da secret"
	token, err := MakeJWT(id, secret, 5 * time.Second)

	parsedID, err := ValidateJWT(token, secret)
	if err != nil {
		t.Errorf("Error: %v", err)
	}
	if id != parsedID {
		t.Errorf("IDs mismatched: %s %s", id.String(), parsedID.String())
	}
}

func TestIDCollission(t *testing.T) {
	token, err := MakeJWT(uuid.New(), "the seecreeeeet", 5 * time.Second)
	if reflect.TypeOf(token).Kind() != reflect.String {
		t.Errorf(`Token: %v Error: %v`, token, err)
	}
	otherToken, err := MakeJWT(uuid.New(), "the seecreeeeet", 5 * time.Second)
	if reflect.TypeOf(otherToken).Kind() != reflect.String {
		t.Errorf(`Token: %v Error: %v`, otherToken, err)
	}
	if token == otherToken {
		t.Errorf(`Tokens %v and %v are the same`, token, otherToken)
	}
}
