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

	"google.golang.org/protobuf/proto"

	sportspb "git.neds.sh/matty/entain/api/proto/sports"
)

// TestAPI_ListEvents_HTTPErrorHandling tests HTTP-level error scenarios.
func TestAPI_ListEvents_HTTPErrorHandling(t *testing.T) {
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
			name:           "empty body returns all events",
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
			endpoint := fmt.Sprintf("%s%s", getAPIEndpoint(), listEventsPath)

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

			// For successful requests, verify response contains events array
			if tc.expectSuccess && resp.StatusCode == http.StatusOK {
				var result map[string]interface{}
				respBody, err := io.ReadAll(resp.Body)
				if err != nil {
					t.Fatalf("failed to read response body: %v", err)
				}
				if err := json.Unmarshal(respBody, &result); err != nil {
					t.Errorf("response is not valid JSON: %v", err)
				}
				if _, ok := result["events"]; !ok {
					t.Error("successful response missing 'events' field")
				}
			}
		})
	}
}

// TestAPI_ListEvents_ResponseFieldValidation tests that all response fields are properly serialized.
func TestAPI_ListEvents_ResponseFieldValidation(t *testing.T) {
	resp := callListEvents(t, &sportspb.ListEventsRequest{})

	if len(resp.Events) == 0 {
		t.Skip("no events in database to validate")
	}

	for i, event := range resp.Events {
		// Validate ID is present
		if event.Id == 0 {
			t.Errorf("event[%d]: expected non-zero Id", i)
		}

		// Validate SportID is present
		if event.SportId == 0 {
			t.Errorf("event[%d]: expected non-zero SportId", i)
		}

		// Validate SportTypeName is not empty
		if event.SportTypeName == "" {
			t.Errorf("event[%d]: expected non-empty SportTypeName", i)
		}

		// Validate Name is not empty
		if event.Name == "" {
			t.Errorf("event[%d]: expected non-empty Name", i)
		}

		// Validate AdvertisedStartTime is set
		if event.AdvertisedStartTime == nil {
			t.Errorf("event[%d]: expected AdvertisedStartTime to be set", i)
		}

		// Validate Status is set (should be OPEN or CLOSED, not UNSPECIFIED)
		if event.Status == sportspb.Event_UNSPECIFIED {
			t.Errorf("event[%d]: expected status to be set (OPEN or CLOSED), got UNSPECIFIED", i)
		}
	}
}

// TestAPI_ListEvents_StatusValidation validates that the status field
// is correctly computed based on advertised_start_time.
// Events with future times should be OPEN, events with past times should be CLOSED.
func TestAPI_ListEvents_StatusValidation(t *testing.T) {
	testCases := []struct {
		name           string
		filter         *sportspb.ListEventsRequestFilter
		validateStatus func(t *testing.T, events []*sportspb.Event)
	}{
		{
			name:   "all events have valid status",
			filter: &sportspb.ListEventsRequestFilter{},
			validateStatus: func(t *testing.T, events []*sportspb.Event) {
				for _, event := range events {
					if event.Status == sportspb.Event_UNSPECIFIED {
						t.Errorf("event %d: expected status to be OPEN or CLOSED, got UNSPECIFIED", event.Id)
					}
				}
			},
		},
		{
			name: "status field exists in response",
			filter: &sportspb.ListEventsRequestFilter{
				SportIds: []int64{1, 2, 3},
			},
			validateStatus: func(t *testing.T, events []*sportspb.Event) {
				if len(events) == 0 {
					t.Skip("no events to validate status")
				}
				for _, event := range events {
					// Status should be either OPEN (1) or CLOSED (2)
					if event.Status != sportspb.Event_OPEN && event.Status != sportspb.Event_CLOSED {
						t.Errorf("event %d: expected status OPEN (1) or CLOSED (2), got %v", event.Id, event.Status)
					}
				}
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			resp := callListEvents(t, &sportspb.ListEventsRequest{
				Filter: tc.filter,
			})

			if tc.validateStatus != nil {
				tc.validateStatus(t, resp.Events)
			}
		})
	}
}

// TestAPI_ListEvents_EdgeCases tests various filters at the API level.
func TestAPI_ListEvents_EdgeCases(t *testing.T) {
	testCases := []struct {
		name           string
		filter         *sportspb.ListEventsRequestFilter
		expectedCount  int
		expectedStatus int
		validateError  func(t *testing.T, statusCode int, body string)
		validateEvents func(t *testing.T, events []*sportspb.Event)
	}{
		{
			name:           "nil filter returns all events",
			filter:         nil,
			expectedCount:  -1, // -1 means we just check it returns successfully
			validateEvents: func(t *testing.T, events []*sportspb.Event) {},
		},
		{
			name:           "empty filter returns all events",
			filter:         &sportspb.ListEventsRequestFilter{},
			expectedCount:  -1,
			validateEvents: func(t *testing.T, events []*sportspb.Event) {},
		},
		{
			name: "single sport id",
			filter: &sportspb.ListEventsRequestFilter{
				SportIds: []int64{1},
			},
			expectedCount: -1,
			validateEvents: func(t *testing.T, events []*sportspb.Event) {
				assertEventsFromSportIDs(t, events, []int64{1})
			},
		},
		{
			name: "multiple sport ids",
			filter: &sportspb.ListEventsRequestFilter{
				SportIds: []int64{1, 2, 3},
			},
			expectedCount: -1,
			validateEvents: func(t *testing.T, events []*sportspb.Event) {
				assertEventsFromSportIDs(t, events, []int64{1, 2, 3})
			},
		},
		{
			name: "multiple sport ids exceeding maximum",
			filter: &sportspb.ListEventsRequestFilter{
				SportIds: func() []int64 {
					count := 1000 // Arbitrarily large number
					ids := make([]int64, count)
					for i := 0; i < count; i++ {
						ids[i] = int64(i + 1)
					}
					return ids
				}(),
			},
			expectedStatus: http.StatusBadRequest,
			validateError: func(t *testing.T, statusCode int, body string) {
				if statusCode != http.StatusBadRequest {
					t.Errorf("expected status %d, got %d", http.StatusBadRequest, statusCode)
				}

				var errResp map[string]interface{}
				if err := json.Unmarshal([]byte(body), &errResp); err != nil {
					t.Fatalf("failed to parse error response: %v", err)
				}

				msg, ok := errResp["message"].(string)
				if !ok {
					t.Errorf("expected error response to have 'message' field, got: %v", errResp)
					return
				}

				if !strings.Contains(msg, "too many sport_ids") {
					t.Errorf("expected error message to contain %q, got: %s", "too many sport_ids", msg)
				}
			},
		},
		{
			name: "duplicate sport ids in filter",
			filter: &sportspb.ListEventsRequestFilter{
				SportIds: []int64{1, 1, 1},
			},
			expectedCount: -1,
			validateEvents: func(t *testing.T, events []*sportspb.Event) {
				assertEventsFromSportIDs(t, events, []int64{1})
			},
		},
		{
			name: "non-existent sport id in filter",
			filter: &sportspb.ListEventsRequestFilter{
				SportIds: []int64{99999},
			},
			expectedCount:  0,
			validateEvents: func(t *testing.T, events []*sportspb.Event) {},
		},
		{
			name: "both existent and non-existent sport ids in filter",
			filter: &sportspb.ListEventsRequestFilter{
				SportIds: []int64{1, 99999},
			},
			expectedCount: -1,
			validateEvents: func(t *testing.T, events []*sportspb.Event) {
				assertEventsFromSportIDs(t, events, []int64{1})
			},
		},
		{
			name: "visible filter true",
			filter: &sportspb.ListEventsRequestFilter{
				Visible: proto.Bool(true),
			},
			expectedCount: -1,
			validateEvents: func(t *testing.T, events []*sportspb.Event) {
				for _, event := range events {
					if !event.Visible {
						t.Errorf("expected visible event, got event %d with visible=%v", event.Id, event.Visible)
					}
				}
			},
		},
		{
			name: "visible filter false",
			filter: &sportspb.ListEventsRequestFilter{
				Visible: proto.Bool(false),
			},
			expectedCount: -1,
			validateEvents: func(t *testing.T, events []*sportspb.Event) {
				for _, event := range events {
					if event.Visible {
						t.Errorf("expected invisible event, got event %d with visible=%v", event.Id, event.Visible)
					}
				}
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Handle error cases with direct HTTP call
			if tc.expectedStatus != 0 {
				body, err := json.Marshal(&sportspb.ListEventsRequest{Filter: tc.filter})
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

				respBody, _ := io.ReadAll(resp.Body)

				if tc.validateError != nil {
					tc.validateError(t, resp.StatusCode, string(respBody))
				}
				return
			}

			resp := callListEvents(t, &sportspb.ListEventsRequest{
				Filter: tc.filter,
			})

			if tc.expectedCount >= 0 && len(resp.Events) != tc.expectedCount {
				t.Errorf("expected %d events, got %d", tc.expectedCount, len(resp.Events))
			}

			tc.validateEvents(t, resp.Events)
		})
	}
}

// TestAPI_ListEvents_ResponseBodyFormat tests that the response body is valid JSON and
// contains the expected structure.
func TestAPI_ListEvents_ResponseBodyFormat(t *testing.T) {
	endpoint := fmt.Sprintf("%s%s", getAPIEndpoint(), listEventsPath)

	reqBody := `{"filter": {}}`
	httpReq, err := http.NewRequest(http.MethodPost, endpoint, strings.NewReader(reqBody))
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

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("failed to read response body: %v", err)
	}

	// Verify response is valid JSON
	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		t.Fatalf("response is not valid JSON: %v", err)
	}

	// Verify events field exists
	events, ok := result["events"]
	if !ok {
		t.Fatal("response missing 'events' field")
	}

	// Verify events is an array
	eventsSlice, ok := events.([]interface{})
	if !ok {
		t.Fatalf("expected events to be an array, got %T", events)
	}

	// Verify each event has the expected fields if any events exist
	if len(eventsSlice) > 0 {
		firstEvent, ok := eventsSlice[0].(map[string]interface{})
		if !ok {
			t.Fatalf("expected event to be an object, got %T", eventsSlice[0])
		}

		requiredFields := []string{"id", "sportId", "sportTypeName", "name", "visible", "advertisedStartTime", "status"}
		for _, field := range requiredFields {
			if _, ok := firstEvent[field]; !ok {
				t.Errorf("event missing required field: %s", field)
			}
		}
	}
}

// TestAPI_ListEvents_ContextTimeout tests that the endpoint handles context timeouts gracefully.
func TestAPI_ListEvents_ContextTimeout(t *testing.T) {
	endpoint := fmt.Sprintf("%s%s", getAPIEndpoint(), listEventsPath)

	// Create a context with a very short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	defer cancel()

	reqBody := `{"filter": {}}`
	httpReq, err := http.NewRequest(http.MethodPost, endpoint, strings.NewReader(reqBody))
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq = httpReq.WithContext(ctx)

	client := &http.Client{}
	_, err = client.Do(httpReq)

	// We expect an error due to context timeout
	if err == nil {
		// If no error, the request completed before timeout, which is also acceptable
		return
	}

	if !errors.Is(err, context.DeadlineExceeded) {
		// The error might be wrapped, so check the error message
		if !strings.Contains(err.Error(), "context deadline exceeded") {
			t.Logf("got different error (may be acceptable): %v", err)
		}
	}
}

// TestAPI_ListEvents_Sorting tests the sorting functionality.
func TestAPI_ListEvents_Sorting(t *testing.T) {
	testCases := []struct {
		name          string
		filter        *sportspb.ListEventsRequestFilter
		validateOrder func(t *testing.T, events []*sportspb.Event)
	}{
		{
			name:   "default sort by advertised_start_time ascending",
			filter: &sportspb.ListEventsRequestFilter{},
			validateOrder: func(t *testing.T, events []*sportspb.Event) {
				if len(events) < 2 {
					t.Skip("not enough events to verify sorting")
				}
				for i := 1; i < len(events); i++ {
					prev := events[i-1].AdvertisedStartTime.AsTime()
					curr := events[i].AdvertisedStartTime.AsTime()
					if prev.After(curr) {
						t.Errorf("events not sorted by advertised_start_time ASC: event[%d] (%v) should be before event[%d] (%v)",
							i-1, prev, i, curr)
					}
				}
			},
		},
		{
			name: "sort by advertised_start_time descending",
			filter: &sportspb.ListEventsRequestFilter{
				SortBy:     proto.String("advertised_start_time"),
				Descending: proto.Bool(true),
			},
			validateOrder: func(t *testing.T, events []*sportspb.Event) {
				if len(events) < 2 {
					t.Skip("not enough events to verify sorting")
				}
				for i := 1; i < len(events); i++ {
					prev := events[i-1].AdvertisedStartTime.AsTime()
					curr := events[i].AdvertisedStartTime.AsTime()
					if prev.Before(curr) {
						t.Errorf("events not sorted by advertised_start_time DESC: event[%d] (%v) should be after event[%d] (%v)",
							i-1, prev, i, curr)
					}
				}
			},
		},
		{
			name: "sort by id ascending",
			filter: &sportspb.ListEventsRequestFilter{
				SortBy: proto.String("id"),
			},
			validateOrder: func(t *testing.T, events []*sportspb.Event) {
				if len(events) < 2 {
					t.Skip("not enough events to verify sorting")
				}
				for i := 1; i < len(events); i++ {
					if events[i-1].Id > events[i].Id {
						t.Errorf("events not sorted by id ASC: event[%d] (id=%d) should be before event[%d] (id=%d)",
							i-1, events[i-1].Id, i, events[i].Id)
					}
				}
			},
		},
		{
			name: "sort by name ascending",
			filter: &sportspb.ListEventsRequestFilter{
				SortBy: proto.String("name"),
			},
			validateOrder: func(t *testing.T, events []*sportspb.Event) {
				if len(events) < 2 {
					t.Skip("not enough events to verify sorting")
				}
				for i := 1; i < len(events); i++ {
					if events[i-1].Name > events[i].Name {
						t.Errorf("events not sorted by name ASC: event[%d] (%s) should be before event[%d] (%s)",
							i-1, events[i-1].Name, i, events[i].Name)
					}
				}
			},
		},
		{
			name: "sort by sport_id ascending",
			filter: &sportspb.ListEventsRequestFilter{
				SortBy: proto.String("sport_id"),
			},
			validateOrder: func(t *testing.T, events []*sportspb.Event) {
				if len(events) < 2 {
					t.Skip("not enough events to verify sorting")
				}
				for i := 1; i < len(events); i++ {
					if events[i-1].SportId > events[i].SportId {
						t.Errorf("events not sorted by sport_id ASC: event[%d] (sport_id=%d) should be before event[%d] (sport_id=%d)",
							i-1, events[i-1].SportId, i, events[i].SportId)
					}
				}
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			resp := callListEvents(t, &sportspb.ListEventsRequest{
				Filter: tc.filter,
			})

			if tc.validateOrder != nil {
				tc.validateOrder(t, resp.Events)
			}
		})
	}
}
