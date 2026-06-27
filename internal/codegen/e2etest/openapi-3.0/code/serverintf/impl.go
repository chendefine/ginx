package serverintf

import (
	"context"
	"fmt"
	"net/http"

	"github.com/chendefine/ginx"
)

type TestService struct {
	pets map[int64]*Pet
}

func NewTestService() *TestService {
	return &TestService{
		pets: map[int64]*Pet{
			1: {ID: 1, Name: "Buddy"},
			2: {ID: 2, Name: "Max"},
		},
	}
}

func (s *TestService) ListPets(_ context.Context, _ *ListPetsReq) (*ListPetsRsp, error) {
	var list []Pet
	for _, p := range s.pets {
		list = append(list, *p)
	}
	result := ListPetsRsp(list)
	return &result, nil
}

func (s *TestService) CreatePet(_ context.Context, req *CreatePetReq) (*CreatePetRsp, error) {
	pet := Pet{ID: req.ID, Name: req.Name}
	s.pets[pet.ID] = &pet
	result := CreatePetRsp(pet)
	return &result, nil
}

func (s *TestService) GetPet(_ context.Context, req *GetPetReq) (*GetPetRsp, error) {
	pet, ok := s.pets[req.PetID]
	if !ok {
		return nil, ginx.Error(404, "pet not found").Status(http.StatusNotFound)
	}
	result := GetPetRsp(*pet)
	return &result, nil
}

func (s *TestService) DeletePet(_ context.Context, req *DeletePetReq) (*struct{}, error) {
	if _, ok := s.pets[req.PetID]; !ok {
		return nil, ginx.Error(404, "pet not found").Status(http.StatusNotFound)
	}
	delete(s.pets, req.PetID)
	return &struct{}{}, nil
}

func (s *TestService) StreamEvents(_ context.Context, req *StreamEventsReq, send ginx.Sender) error {
	for i := 1; i <= 2; i++ {
		if err := send(ginx.Event{
			ID:    fmt.Sprintf("%d", i),
			Event: "msg",
			Data:  fmt.Sprintf(`{"ch":"%s","i":%d}`, req.Channel, i),
		}); err != nil {
			return err
		}
	}
	return nil
}

func (s *TestService) StreamNotifications(_ context.Context, req *StreamNotificationsReq, send ginx.Sender) error {
	return send(ginx.Event{ID: "1", Event: "notify", Data: "hello"})
}

var _ ServerInterface = (*TestService)(nil)
