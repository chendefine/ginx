package ginx

import (
	"errors"
	"reflect"
	"testing"
)

func TestValidateResponseStatus(t *testing.T) {
	if err := ValidateResponseStatus(202, 200, 202, 204); err != nil {
		t.Fatalf("expected accepted status: %v", err)
	}
	expected := []int{201, 202}
	err := ValidateResponseStatus(200, expected...)
	var statusErr *UnexpectedStatusError
	if !errors.As(err, &statusErr) {
		t.Fatalf("expected *UnexpectedStatusError, got %T", err)
	}
	if statusErr.StatusCode != 200 || !reflect.DeepEqual(statusErr.Expected, []int{201, 202}) {
		t.Fatalf("unexpected error: %+v", statusErr)
	}
	expected[0] = 500
	if !reflect.DeepEqual(statusErr.Expected, []int{201, 202}) {
		t.Fatalf("Expected was not copied: %v", statusErr.Expected)
	}
}

func TestParseResponse_EmptyBody_Success(t *testing.T) {
	err := ParseResponse(200, nil, nil)
	if err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
}

func TestParseResponse_EmptyBody_HTTPError(t *testing.T) {
	err := ParseResponse(500, nil, nil)
	if err == nil {
		t.Fatal("expected error")
	}
	var e *ErrWrap
	if !errors.As(err, &e) {
		t.Fatalf("expected *ErrWrap, got %T", err)
	}
	if e.HttpCode != 500 {
		t.Errorf("expected HttpCode=500, got %d", e.HttpCode)
	}
}

func TestParseResponse_DataWrap_Success(t *testing.T) {
	body := []byte(`{"code":0,"msg":"ok","data":{"name":"test"}}`)
	var result struct {
		Name string `json:"name"`
	}
	err := ParseResponse(200, body, &result)
	if err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
	if result.Name != "test" {
		t.Errorf("expected name=test, got %q", result.Name)
	}
}

func TestParseResponse_DataWrap_BusinessError(t *testing.T) {
	body := []byte(`{"code":1001,"msg":"not found"}`)
	err := ParseResponse(200, body, nil)
	if err == nil {
		t.Fatal("expected error")
	}
	var e *ErrWrap
	if !errors.As(err, &e) {
		t.Fatalf("expected *ErrWrap, got %T", err)
	}
	if e.Code != 1001 {
		t.Errorf("expected Code=1001, got %d", e.Code)
	}
	if e.Msg != "not found" {
		t.Errorf("expected Msg='not found', got %q", e.Msg)
	}
	if e.HttpCode != 200 {
		t.Errorf("expected HttpCode=200, got %d", e.HttpCode)
	}
}

func TestParseResponse_DataWrap_BusinessError_HTTP500(t *testing.T) {
	body := []byte(`{"code":2000,"msg":"internal error"}`)
	err := ParseResponse(500, body, nil)
	if err == nil {
		t.Fatal("expected error")
	}
	var e *ErrWrap
	if !errors.As(err, &e) {
		t.Fatalf("expected *ErrWrap, got %T", err)
	}
	if e.Code != 2000 {
		t.Errorf("expected Code=2000, got %d", e.Code)
	}
	if e.HttpCode != 500 {
		t.Errorf("expected HttpCode=500, got %d", e.HttpCode)
	}
}

func TestParseResponse_DataWrap_ZeroBusinessCodeHTTPError(t *testing.T) {
	err := ParseResponse(500, []byte(`{"code":0,"msg":"unexpected success envelope","data":{"id":1}}`), nil)
	var e *ErrWrap
	if !errors.As(err, &e) {
		t.Fatalf("expected *ErrWrap, got %T (%v)", err, err)
	}
	if e.Code != -1 || e.HttpCode != 500 || e.Msg != "unexpected success envelope" {
		t.Fatalf("unexpected error: %+v", e)
	}
}

func TestParseResponse_NoDataWrap_Success(t *testing.T) {
	body := []byte(`{"name":"direct"}`)
	var result struct {
		Name string `json:"name"`
	}
	err := ParseResponse(200, body, &result)
	if err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
	if result.Name != "direct" {
		t.Errorf("expected name=direct, got %q", result.Name)
	}
}

func TestParseResponse_NoDataWrap_CodeFieldOnly(t *testing.T) {
	body := []byte(`{"code":0,"name":"direct"}`)
	var result struct {
		Code int    `json:"code"`
		Name string `json:"name"`
	}
	err := ParseResponse(200, body, &result)
	if err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
	if result.Code != 0 || result.Name != "direct" {
		t.Errorf("unexpected result: %+v", result)
	}
}

func TestParseResponse_NoDataWrap_NonZeroCodeFieldOnly(t *testing.T) {
	body := []byte(`{"code":42,"name":"direct"}`)
	var result struct {
		Code int    `json:"code"`
		Name string `json:"name"`
	}
	err := ParseResponse(200, body, &result)
	if err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
	if result.Code != 42 || result.Name != "direct" {
		t.Errorf("unexpected result: %+v", result)
	}
}

func TestParseResponse_DataWrap_NullData(t *testing.T) {
	body := []byte(`{"code":0,"msg":"ok","data":null}`)
	var result *struct {
		Name string `json:"name"`
	}
	err := ParseResponse(200, body, &result)
	if err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
	if result != nil {
		t.Errorf("expected nil result, got %+v", result)
	}
}

func TestParseResponse_DataWrap_ZeroCodeWithoutData(t *testing.T) {
	body := []byte(`{"code":0,"msg":"ok"}`)
	var result struct {
		Name string `json:"name"`
	}
	err := ParseResponse(200, body, &result)
	if err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
	if result.Name != "" {
		t.Errorf("expected zero result, got %+v", result)
	}
}

func TestParseResponse_NoDataWrap_HTTPError(t *testing.T) {
	body := []byte(`some error text`)
	err := ParseResponse(502, body, nil)
	if err == nil {
		t.Fatal("expected error")
	}
	var e *ErrWrap
	if !errors.As(err, &e) {
		t.Fatalf("expected *ErrWrap, got %T", err)
	}
	if e.HttpCode != 502 {
		t.Errorf("expected HttpCode=502, got %d", e.HttpCode)
	}
	if e.Msg != "some error text" {
		t.Errorf("expected Msg='some error text', got %q", e.Msg)
	}
}

func TestParseResponse_DataWrap_NilResult(t *testing.T) {
	body := []byte(`{"code":0,"msg":"ok","data":{"id":1}}`)
	err := ParseResponse(200, body, nil)
	if err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
}

func TestParseResponse_ArrayResponse(t *testing.T) {
	body := []byte(`{"code":0,"msg":"ok","data":[{"name":"a"},{"name":"b"}]}`)
	var result []struct {
		Name string `json:"name"`
	}
	err := ParseResponse(200, body, &result)
	if err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
	if len(result) != 2 {
		t.Errorf("expected 2 items, got %d", len(result))
	}
}

func TestParseResponse_InvalidWrapperCodeFallsBackToBody(t *testing.T) {
	body := []byte(`{"code":"ok","msg":"not a wrapper","name":"direct"}`)
	var result struct {
		Code string `json:"code"`
		Msg  string `json:"msg"`
		Name string `json:"name"`
	}
	err := ParseResponse(200, body, &result)
	if err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
	if result.Code != "ok" || result.Msg != "not a wrapper" || result.Name != "direct" {
		t.Errorf("unexpected result: %+v", result)
	}
}
