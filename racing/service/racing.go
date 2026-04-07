package service

import (
	"strings"

	"git.neds.sh/matty/entain/racing/db"
	"git.neds.sh/matty/entain/racing/proto/racing"
	"golang.org/x/net/context"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Racing interface {
	// ListRaces will return a collection of races.
	ListRaces(ctx context.Context, in *racing.ListRacesRequest) (*racing.ListRacesResponse, error)

	// GetRace returns a single race by its ID.
	GetRace(ctx context.Context, in *racing.GetRaceRequest) (*racing.GetRaceResponse, error)
}

// racingService implements the Racing interface.
type racingService struct {
	racesRepo db.RacesRepo
}

// NewRacingService instantiates and returns a new racingService.
func NewRacingService(racesRepo db.RacesRepo) Racing {
	return &racingService{racesRepo}
}

func (s *racingService) ListRaces(ctx context.Context, in *racing.ListRacesRequest) (*racing.ListRacesResponse, error) {
	races, err := s.racesRepo.List(in.Filter)
	if err != nil {
		return nil, err
	}

	return &racing.ListRacesResponse{Races: races}, nil
}

func (s *racingService) GetRace(ctx context.Context, in *racing.GetRaceRequest) (*racing.GetRaceResponse, error) {
	// Validate input: ID must be positive
	if in.Id <= 0 {
		return nil, status.Errorf(codes.InvalidArgument, "race id must be a positive integer, got %d", in.Id)
	}

	race, err := s.racesRepo.Get(in.Id)
	if err != nil {
		// Check if the error is "not found"
		if strings.Contains(err.Error(), "not found") {
			return nil, status.Errorf(codes.NotFound, "race with id %d not found", in.Id)
		}
		// Wrap other errors as internal errors
		return nil, status.Errorf(codes.Internal, "failed to get race: %v", err)
	}

	return &racing.GetRaceResponse{Race: race}, nil
}
