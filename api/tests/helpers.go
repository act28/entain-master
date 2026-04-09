//go:build integration

package tests

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"testing"
	"time"

	racingpb "git.neds.sh/matty/entain/api/proto/racing"
	sportspb "git.neds.sh/matty/entain/api/proto/sports"
	"google.golang.org/protobuf/encoding/protojson"
)

const defaultAPIEndpoint = "http://localhost:8000"
const defaultHTTPTimeout = 3 * time.Second

func getAPIEndpoint() string {
	if ep := os.Getenv("API_ENDPOINT"); ep != "" {
		return ep
	}
	return defaultAPIEndpoint
}

const listRacesPath = "/v1/list-races"

func callListRaces(t *testing.T, req *racingpb.ListRacesRequest) *racingpb.ListRacesResponse {
	t.Helper()

	body, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("failed to marshal request: %v", err)
	}

	endpoint := fmt.Sprintf("%s%s", getAPIEndpoint(), listRacesPath)

	httpReq, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: defaultHTTPTimeout}
	resp, err := client.Do(httpReq)
	if err != nil {
		t.Fatalf("failed to send request: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("failed to read response body: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d. Body: %s", resp.StatusCode, string(respBody))
	}

	var result racingpb.ListRacesResponse
	if err := protojson.Unmarshal(respBody, &result); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	return &result
}

func assertRacesFromMeetings(t *testing.T, races []*racingpb.Race, meetingIDs []int64) {
	t.Helper()
	meetingSet := make(map[int64]bool, len(meetingIDs))
	for _, id := range meetingIDs {
		meetingSet[id] = true
	}
	for _, r := range races {
		if !meetingSet[r.MeetingId] {
			t.Errorf("race %d: expected meeting_id in %v, got %d", r.Id, meetingIDs, r.MeetingId)
		}
	}
}

func assertRacesNotFromMeetings(t *testing.T, races []*racingpb.Race, meetingIDs []int64) {
	t.Helper()

	meetingSet := make(map[int64]bool, len(meetingIDs))
	for _, id := range meetingIDs {
		meetingSet[id] = true
	}
	for _, r := range races {
		if meetingSet[r.MeetingId] {
			t.Errorf("race %d: expected meeting_id NOT in %v, got %d", r.Id, meetingIDs, r.MeetingId)
		}
	}
}

// Sports helpers

const listEventsPath = "/v1/sports/events"

func callListEvents(t *testing.T, req *sportspb.ListEventsRequest) *sportspb.ListEventsResponse {
	t.Helper()

	body, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("failed to marshal request: %v", err)
	}

	endpoint := fmt.Sprintf("%s%s", getAPIEndpoint(), listEventsPath)

	httpReq, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		t.Fatalf("failed to send request: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("failed to read response body: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d. Body: %s", resp.StatusCode, string(respBody))
	}

	var result sportspb.ListEventsResponse
	if err := protojson.Unmarshal(respBody, &result); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	return &result
}

func assertEventsFromSportIDs(t *testing.T, events []*sportspb.Event, sportIDs []int64) {
	t.Helper()
	sportSet := make(map[int64]bool, len(sportIDs))
	for _, id := range sportIDs {
		sportSet[id] = true
	}
	for _, e := range events {
		if !sportSet[e.SportId] {
			t.Errorf("event %d: expected sport_id in %v, got %d", e.Id, sportIDs, e.SportId)
		}
	}
}
