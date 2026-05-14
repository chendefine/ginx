package servername

import "context"

type TestService struct{}

func (s *TestService) ListOrders(_ context.Context, _ *ListOrdersReq) (*ListOrdersRsp, error) {
	var result ListOrdersRsp
	result = append(result, map[string]any{"id": "order-1"}, map[string]any{"id": "order-2"})
	return &result, nil
}

func (s *TestService) CreateOrder(_ context.Context, req *CreateOrderReq) (*CreateOrderRsp, error) {
	return &CreateOrderRsp{ID: strPtr("new-order")}, nil
}

var _ ServerInterface = (*TestService)(nil)

func strPtr(s string) *string { return &s }
