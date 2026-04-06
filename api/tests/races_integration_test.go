//go:build integration

package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	racingpb "git.neds.sh/matty/entain/api/proto/racing"
)

// TestAPI_ListRaces_HTTPErrorHandling tests HTTP-level error scenarios.
func TestAPI_ListRaces_HTTPErrorHandling(t *testing.T) {
	testCases := []struct {
		name           string
		method         string
		body           string
		contentType    string
		expectedStatus int
		// expectSuccess indicates if we expect a 200 response
		expectSuccess bool
	}{
		{
			name:           "GET method not allowed",
			method:         http.MethodGet,
			body:           "",
			contentType:    "application/json",
			expectedStatus: http.StatusNotImplemented,
			expectSuccess:  false,
		},
		{
			name:           "invalid JSON body",
			method:         http.MethodPost,
			body:           `{invalid json}`,
			contentType:    "application/json",
			expectedStatus: http.StatusBadRequest,
			expectSuccess:  false,
		},
		{
			name:           "empty body returns all races",
			method:         http.MethodPost,
			body:           "",
			contentType:    "application/json",
			expectedStatus: http.StatusOK,
			expectSuccess:  true,
		},
		{
			name:           "wrong content type",
			method:         http.MethodPost,
			body:           `{"filter": {}}`,
			contentType:    "text/plain",
			expectedStatus: http.StatusOK,
			expectSuccess:  true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			endpoint := fmt.Sprintf("%s%s", getAPIEndpoint(), listRacesPath)

			var bodyReader io.Reader
			if tc.body != "" {
				bodyReader = strings.NewReader(tc.body)
			}

			httpReq, err := http.NewRequest(tc.method, endpoint, bodyReader)
			if err != nil {
				t.Fatalf("failed to create request: %v", err)
			}
			if tc.contentType != "" {
				httpReq.Header.Set("Content-Type", tc.contentType)
			}

			client := &http.Client{Timeout: 10 * time.Second}
			resp, err := client.Do(httpReq)
			if err != nil {
				t.Fatalf("failed to send request: %v", err)
			}
			defer func() { _ = resp.Body.Close() }()

			if resp.StatusCode != tc.expectedStatus {
				body, _ := io.ReadAll(resp.Body)
				t.Errorf("expected status %d, got %d. Body: %s", tc.expectedStatus, resp.StatusCode, string(body))
			}

			// For successful requests, verify response contains races array
			if tc.expectSuccess && resp.StatusCode == http.StatusOK {
				var result map[string]interface{}
				respBody, err := io.ReadAll(resp.Body)
				if err != nil {
					t.Fatalf("failed to read response body: %v", err)
				}
				if err := json.Unmarshal(respBody, &result); err != nil {
					t.Errorf("response is not valid JSON: %v", err)
				}
				if _, ok := result["races"]; !ok {
					t.Error("successful response missing 'races' field")
				}
			}
		})
	}
}

// TestAPI_ListRaces_ResponseFieldValidation tests that all response fields are properly serialized.
func TestAPI_ListRaces_ResponseFieldValidation(t *testing.T) {
	resp := callListRaces(t, &racingpb.ListRacesRequest{})

	if len(resp.Races) == 0 {
		t.Skip("no races in database to validate")
	}

	for i, race := range resp.Races {
		// Validate ID is present
		if race.Id == 0 {
			t.Errorf("race[%d]: expected non-zero Id", i)
		}

		// Validate MeetingID is present
		if race.MeetingId == 0 {
			t.Errorf("race[%d]: expected non-zero MeetingId", i)
		}

		// Validate Name is not empty
		if race.Name == "" {
			t.Errorf("race[%d]: expected non-empty Name", i)
		}

		// Validate Number is present
		if race.Number == 0 {
			t.Errorf("race[%d]: expected non-zero Number", i)
		}

		// Validate AdvertisedStartTime is set
		if race.AdvertisedStartTime == nil {
			t.Errorf("race[%d]: expected AdvertisedStartTime to be set", i)
		}
	}
}

// TestApi_ListRaces_Filters tests various filters at the API level.
func TestAPI_ListRaces_EdgeCases(t *testing.T) {
	testCases := []struct {
		name          string
		filter        *racingpb.ListRacesRequestFilter
		expectedCount int
		validateRaces func(t *testing.T, races []*racingpb.Race)
	}{
		{
			name:          "nil filter returns all races",
			filter:        nil,
			expectedCount: -1, // -1 means we just check it returns successfully
			validateRaces: func(t *testing.T, races []*racingpb.Race) {},
		},
		{
			name:          "empty filter returns all races",
			filter:        &racingpb.ListRacesRequestFilter{},
			expectedCount: -1,
			validateRaces: func(t *testing.T, races []*racingpb.Race) {},
		},
		{
			name: "single meeting id",
			filter: &racingpb.ListRacesRequestFilter{
				MeetingIds: []int64{1},
			},
			expectedCount: -1,
			validateRaces: func(t *testing.T, races []*racingpb.Race) {
				assertRacesFromMeetings(t, races, []int64{1})
			},
		},
		{
			name: "multiple meeting ids",
			filter: &racingpb.ListRacesRequestFilter{
				MeetingIds: []int64{1, 2, 3},
			},
			expectedCount: -1,
			validateRaces: func(t *testing.T, races []*racingpb.Race) {
				assertRacesFromMeetings(t, races, []int64{1, 2, 3})
			},
		},
		{
			name: "duplicate meeting ids in filter",
			filter: &racingpb.ListRacesRequestFilter{
				MeetingIds: []int64{1, 1, 1},
			},
			expectedCount: -1,
			validateRaces: func(t *testing.T, races []*racingpb.Race) {
				assertRacesFromMeetings(t, races, []int64{1})
			},
		},
		{
			name: "non-existent meeting id in filter",
			filter: &racingpb.ListRacesRequestFilter{
				MeetingIds: []int64{99999},
			},
			expectedCount: 0,
			validateRaces: func(t *testing.T, races []*racingpb.Race) {},
		},
		{
			name: "both existent and non-existent meeting ids in filter",
			filter: &racingpb.ListRacesRequestFilter{
				MeetingIds: []int64{1, 99999},
			},
			expectedCount: -1,
			validateRaces: func(t *testing.T, races []*racingpb.Race) {
				assertRacesFromMeetings(t, races, []int64{1})
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			resp := callListRaces(t, &racingpb.ListRacesRequest{
				Filter: tc.filter,
			})

			if tc.expectedCount >= 0 && len(resp.Races) != tc.expectedCount {
				t.Errorf("expected %d races, got %d", tc.expectedCount, len(resp.Races))
			}

			if tc.validateRaces != nil {
				tc.validateRaces(t, resp.Races)
			}
		})
	}
}

// TestAPI_ListRaces_ResponseBodyFormat tests the response body format and JSON serialization.
func TestAPI_ListRaces_ResponseBodyFormat(t *testing.T) {
	endpoint := fmt.Sprintf("%s%s", getAPIEndpoint(), listRacesPath)

	httpReq, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader([]byte(`{"filter": {}}`)))
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

	// Check Content-Type header
	contentType := resp.Header.Get("Content-Type")
	if !strings.Contains(contentType, "application/json") {
		t.Errorf("expected Content-Type to contain 'application/json', got %q", contentType)
	}

	// Read and validate response body format
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("failed to read response body: %v", err)
	}

	// Verify it's valid JSON
	var rawJSON map[string]interface{}
	if err := json.Unmarshal(respBody, &rawJSON); err != nil {
		t.Fatalf("response is not valid JSON: %v", err)
	}

	// Verify response has 'races' field
	if _, ok := rawJSON["races"]; !ok {
		t.Error("response missing 'races' field")
	}
}

// TestAPI_ListRaces_ContextTimeout tests context timeouts are correctly handled.
func TestListRaces_ContextTimeout(t *testing.T) {
	endpoint := fmt.Sprintf("%s%s", getAPIEndpoint(), listRacesPath)

	httpReq, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader([]byte(`{"filter": {}}`)))
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	httpReq = httpReq.WithContext(ctx)

	client := &http.Client{}
	start := time.Now()
	_, err = client.Do(httpReq)
	elapsed := time.Since(start)

	// ASSERTION: Should complete (either success or context deadline exceeded)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			t.Logf("request timed out as expected after %v", elapsed)
			// Verify timeout happened within reasonable window
			if elapsed < 1*time.Second || elapsed > 3*time.Second {
				t.Errorf("timeout occurred at %v, expected around 1 second", elapsed)
			}
			return
		}
		t.Fatalf("unexpected error: %v", err)
	}

	t.Logf("request completed successfully in %v (before timeout)", elapsed)
}
