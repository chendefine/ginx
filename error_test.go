package ginx

import (
	"net/http"
	"testing"
)

func TestStatusReturnsNewInstanceAndKeepsBaseUnchanged(t *testing.T) {
	base := Error(100, "msg")
	updated := base.Status(http.StatusCreated)
	if updated == base {
		t.Fatal("Status should return a new instance")
	}
	if base.HttpCode != 0 {
		t.Fatalf("base.HttpCode=%d", base.HttpCode)
	}
	if updated.HttpCode != http.StatusCreated {
		t.Fatalf("updated.HttpCode=%d", updated.HttpCode)
	}
}

func TestErrorConstructor(t *testing.T) {
	ew := Error(100, "something failed")
	if ew.Code != 100 {
		t.Fatalf("Code=%d", ew.Code)
	}
	if ew.Msg != "something failed" {
		t.Fatalf("Msg=%q", ew.Msg)
	}
	if ew.HttpCode != 0 {
		t.Fatalf("HttpCode=%d", ew.HttpCode)
	}
}

func TestStatusSetsHTTPCode(t *testing.T) {
	ew := Error(100, "not found").Status(404)
	if ew.HttpCode != 404 {
		t.Fatalf("HttpCode=%d", ew.HttpCode)
	}
}

func TestStatusIgnoresInvalidHTTPCode(t *testing.T) {
	for _, code := range []int{0, 100, 600} {
		ew := Error(100, "msg").Status(code)
		if ew.HttpCode != 0 {
			t.Fatalf("code=%d httpCode=%d", code, ew.HttpCode)
		}
	}
}

func TestErrWrapErrorString(t *testing.T) {
	ew := &ErrWrap{Code: 42, Msg: "test error"}
	if ew.Error() != "test error" {
		t.Fatalf("Error()=%q", ew.Error())
	}
}

func TestFormatReturnsNewError(t *testing.T) {
	base := Error(1001, "user %s not found, id=%d").Status(403)
	formatted := base.Format("alice", 42)

	if formatted.Msg != "user alice not found, id=42" {
		t.Fatalf("Msg=%q", formatted.Msg)
	}
	if formatted.Code != 1001 || formatted.HttpCode != 403 {
		t.Fatalf("formatted=%+v", formatted)
	}
	if base.Msg != "user %s not found, id=%d" {
		t.Fatalf("base mutated=%q", base.Msg)
	}
}
