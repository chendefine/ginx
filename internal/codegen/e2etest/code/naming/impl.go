package naming

import "context"

type TestService struct{}

func (s *TestService) ListHTTPAPIEndpoints(_ context.Context, req *ListHTTPAPIEndpointsReq) (*ListHTTPAPIEndpointsRsp, error) {
	url := "https://api.example.com/" + req.APIID
	var result ListHTTPAPIEndpointsRsp
	result = append(result, map[string]any{"url": url})
	return &result, nil
}

func (s *TestService) GetNoOperationIDResourceID(_ context.Context, req *GetNoOperationIDResourceIDReq) (*GetNoOperationIDResourceIDRsp, error) {
	return &GetNoOperationIDResourceIDRsp{ID: &req.ResourceID}, nil
}

func (s *TestService) PostNoOperationIDResourceID(_ context.Context, req *PostNoOperationIDResourceIDReq) (*PostNoOperationIDResourceIDRsp, error) {
	return &PostNoOperationIDResourceIDRsp{ID: &req.ResourceID}, nil
}

var _ ServerInterface = (*TestService)(nil)
