package service

import (
	"context"
	"fmt"

	"git.neds.sh/matty/entain/sports/db"
	"git.neds.sh/matty/entain/sports/proto/sports"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// MaxSportIDs is the maximum number of sport IDs allowed in a filter.
const MaxSportIDs = 5

type Sports interface {
	// ListEvents will return a collection of events.
	ListEvents(ctx context.Context, in *sports.ListEventsRequest) (*sports.ListEventsResponse, error)
}

// sportsService implements the Sports interface.
type sportsService struct {
	eventsRepo db.EventsRepo
}

// NewSportsService instantiates and returns a new sportsService.
func NewSportsService(eventsRepo db.EventsRepo) Sports {
	return &sportsService{eventsRepo}
}

func (s *sportsService) ListEvents(ctx context.Context, in *sports.ListEventsRequest) (*sports.ListEventsResponse, error) {
	// Validate filter if provided.
	if in.Filter != nil {
		if err := s.validateFilter(in.Filter); err != nil {
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}
	}

	events, err := s.eventsRepo.List(ctx, in.Filter)
	if err != nil {
		return nil, status.Errorf(codes.Internal, err.Error())
	}

	return &sports.ListEventsResponse{Events: events}, nil
}

// validateFilter validates the filter parameters.
// Returns an error if the filter contains invalid values.
func (s *sportsService) validateFilter(filter *sports.ListEventsRequestFilter) error {
	// Validate sport_ids if provided.
	if len(filter.SportIds) > 0 {
		// Check maximum number of sport IDs first.
		if len(filter.SportIds) > MaxSportIDs {
			return fmt.Errorf("too many sport_ids: got %d, maximum allowed is %d", len(filter.SportIds), MaxSportIDs)
		}

		for _, id := range filter.SportIds {
			if id <= 0 {
				return fmt.Errorf("sport_id must be positive, got %d", id)
			}
		}
	}

	// Note: sort_by validation is handled safely in the db layer via whitelist.
	// This avoids exposing internal database schema details.

	return nil
}
