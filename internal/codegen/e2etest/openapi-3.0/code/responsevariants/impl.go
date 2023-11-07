package responsevariants

import (
	"context"
	"net/http"

	"github.com/chendefine/ginx"
)

type TestService struct{}

func (s *TestService) CreateJob(_ context.Context, req *CreateJobReq) (*CreateJobResponse, error) {
	switch req.Outcome {
	case "201":
		return NewCreateJob201Response(&CreateJob201Rsp{ID: "resource-1"}), nil
	case "202":
		return NewCreateJob202Response(&CreateJob202Rsp{JobID: "job-1"}), nil
	case "204":
		return NewCreateJob204Response(), nil
	default:
		return nil, &ginx.ErrWrap{Code: 40001, Msg: "invalid outcome", HttpCode: http.StatusBadRequest}
	}
}

var _ ServerInterface = (*TestService)(nil)
