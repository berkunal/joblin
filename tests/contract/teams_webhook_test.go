package contract

import (
	"context"
	"encoding/json"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test constants to reduce duplication
const (
	testJobID1           = "12345678-1234-4234-8234-123456789012"
	testJobID2           = "12345678-1234-4234-8234-123456789013"
	testJobID3           = "12345678-1234-4234-8234-123456789014"
	testJobID4           = "12345678-1234-4234-8234-123456789015"
	testScriptPy         = "test_script.py"
	testCluster          = "test-cluster"
	testWebhookURL       = "https://teams.microsoft.com/webhook/test"
	schemaContext        = "http://schema.org/extensions"
	jobIDFieldName       = "Job ID"
	moduleNotFoundError  = "ModuleNotFoundError: No module named 'nonexistent'"
)

// TestTeamsWebhookNotifications tests the contract for Teams webhook notifications
// This test MUST FAIL until the Teams webhook integration is implemented
func TestTeamsWebhookNotifications(t *testing.T) {
	tests := []struct {
		name                string
		jobSpec             JobNotification
		expectedMessageCard TeamsMessageCard
		expectError         string
		serverResponse      int
		serverDelay         time.Duration
	}{
		{
			name: "success_notification",
			jobSpec: JobNotification{
				JobID:           testJobID1,
				Name:            "test-job",
				ScriptFilename:  testScriptPy,
				Status:          "completed",
				Duration:        "2m15s",
				KubernetesContext: testCluster,
				Namespace:       "default",
				ExitCode:        0,
				CompletedAt:     time.Date(2025, 9, 15, 10, 35, 0, 0, time.UTC),
				WebhookURL:      testWebhookURL,
			},
			expectedMessageCard: TeamsMessageCard{
				Type:       "MessageCard",
				Context:    schemaContext,
				ThemeColor: "00FF00",
				Summary:    "Joblin Job Completed Successfully",
				Sections: []MessageSection{
					{
						ActivityTitle:    "✅ Job Completed Successfully",
						ActivitySubtitle: "Job: test-job",
						ActivityImage:    "https://via.placeholder.com/64x64/00FF00/FFFFFF?text=✓",
						Facts: []MessageFact{
							{Name: jobIDFieldName, Value: testJobID1},
							{Name: "Script", Value: testScriptPy},
							{Name: "Duration", Value: "2m15s"},
							{Name: "Cluster", Value: testCluster},
							{Name: "Namespace", Value: "default"},
							{Name: "Exit Code", Value: "0"},
							{Name: "Completed At", Value: "2025-09-15T10:35:00Z"},
						},
						Markdown: true,
					},
				},
			},
			serverResponse: http.StatusOK,
		},
		{
			name: "failure_notification",
			jobSpec: JobNotification{
				JobID:           testJobID2,
				Name:            "failing-job",
				ScriptFilename:  "failing_script.py",
				Status:          "failed",
				Duration:        "1m30s",
				KubernetesContext: testCluster,
				Namespace:       "default",
				ExitCode:        1,
				CompletedAt:     time.Date(2025, 9, 15, 10, 37, 0, 0, time.UTC),
				ErrorMessage:    moduleNotFoundError,
				LastLogLines:    []string{"Traceback (most recent call last):", "  File \"failing_script.py\", line 1, in <module>", "    import nonexistent", moduleNotFoundError},
				WebhookURL:      testWebhookURL,
			},
			expectedMessageCard: TeamsMessageCard{
				Type:       "MessageCard",
				Context:    schemaContext,
				ThemeColor: "FF0000",
				Summary:    "Joblin Job Failed",
				Sections: []MessageSection{
					{
						ActivityTitle:    "❌ Job Failed",
						ActivitySubtitle: "Job: failing-job",
						ActivityImage:    "https://via.placeholder.com/64x64/FF0000/FFFFFF?text=✗",
						Facts: []MessageFact{
							{Name: jobIDFieldName, Value: testJobID2},
							{Name: "Script", Value: "failing_script.py"},
							{Name: "Duration", Value: "1m30s"},
							{Name: "Cluster", Value: testCluster},
							{Name: "Namespace", Value: "default"},
							{Name: "Exit Code", Value: "1"},
							{Name: "Failed At", Value: "2025-09-15T10:37:00Z"},
							{Name: "Error Summary", Value: moduleNotFoundError},
						},
						Markdown: true,
					},
					{
						Title: "Last Log Entries",
						Text:  "```\nTraceback (most recent call last):\n  File \"failing_script.py\", line 1, in <module>\n    import nonexistent\nModuleNotFoundError: No module named 'nonexistent'\n```",
						Markdown: true,
					},
				},
			},
			serverResponse: http.StatusOK,
		},
		{
			name: "termination_notification",
			jobSpec: JobNotification{
				JobID:           testJobID3,
				Name:            "terminated-job",
				ScriptFilename:  "long_running.py",
				Status:          "terminated",
				Duration:        "45s",
				KubernetesContext: testCluster,
				Namespace:       "default",
				TerminatedAt:    time.Date(2025, 9, 15, 10, 40, 0, 0, time.UTC),
				TerminatedBy:    "user@example.com",
				WebhookURL:      testWebhookURL,
			},
			expectedMessageCard: TeamsMessageCard{
				Type:       "MessageCard",
				Context:    schemaContext,
				ThemeColor: "FFA500",
				Summary:    "Joblin Job Terminated",
				Sections: []MessageSection{
					{
						ActivityTitle:    "⚠️ Job Terminated by User",
						ActivitySubtitle: "Job: terminated-job",
						ActivityImage:    "https://via.placeholder.com/64x64/FFA500/FFFFFF?text=⚠",
						Facts: []MessageFact{
							{Name: jobIDFieldName, Value: testJobID3},
							{Name: "Script", Value: "long_running.py"},
							{Name: "Runtime", Value: "45s"},
							{Name: "Cluster", Value: testCluster},
							{Name: "Namespace", Value: "default"},
							{Name: "Terminated At", Value: "2025-09-15T10:40:00Z"},
							{Name: "Terminated By", Value: "user@example.com"},
						},
						Markdown: true,
					},
				},
			},
			serverResponse: http.StatusOK,
		},
		{
			name: "rate_limited_response",
			jobSpec: JobNotification{
				JobID:           "12345678-1234-4234-8234-123456789015",
				Name:            "rate-limited-job",
				ScriptFilename:  testScriptPy,
				Status:          "completed",
				Duration:        "1m",
				KubernetesContext: testCluster,
				Namespace:       "default",
				ExitCode:        0,
				CompletedAt:     time.Date(2025, 9, 15, 10, 35, 0, 0, time.UTC),
				WebhookURL:      testWebhookURL,
			},
			serverResponse: http.StatusTooManyRequests,
			expectError:    "rate limited",
		},
		{
			name: "invalid_webhook_url",
			jobSpec: JobNotification{
				JobID:           "12345678-1234-4234-8234-123456789016",
				Name:            "invalid-webhook-job",
				ScriptFilename:  testScriptPy,
				Status:          "completed",
				Duration:        "1m",
				KubernetesContext: testCluster,
				Namespace:       "default",
				ExitCode:        0,
				CompletedAt:     time.Date(2025, 9, 15, 10, 35, 0, 0, time.UTC),
				WebhookURL:      "https://teams.microsoft.com/webhook/expired",
			},
			serverResponse: http.StatusBadRequest,
			expectError:    "invalid webhook URL",
		},
		{
			name: "timeout_scenario",
			jobSpec: JobNotification{
				JobID:           "12345678-1234-4234-8234-123456789017",
				Name:            "timeout-job",
				ScriptFilename:  testScriptPy,
				Status:          "completed",
				Duration:        "1m",
				KubernetesContext: testCluster,
				Namespace:       "default",
				ExitCode:        0,
				CompletedAt:     time.Date(2025, 9, 15, 10, 35, 0, 0, time.UTC),
				WebhookURL:      "https://teams.microsoft.com/webhook/timeout",
			},
			serverResponse: http.StatusOK,
			serverDelay:    35 * time.Second, // Longer than 30s timeout
			expectError:    "timeout",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock Teams webhook server
			var receivedPayload TeamsMessageCard
			var receivedHeaders http.Header
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Add delay if specified
				if tt.serverDelay > 0 {
					time.Sleep(tt.serverDelay)
				}

				// Capture headers
				receivedHeaders = r.Header.Clone()

				// Read and parse the request body
				body, err := io.ReadAll(r.Body)
				require.NoError(t, err, "Failed to read request body")

				if tt.serverResponse == http.StatusOK {
					err = json.Unmarshal(body, &receivedPayload)
					assert.NoError(t, err, "Failed to parse request JSON")
				}

				// Send response
				w.WriteHeader(tt.serverResponse)
				if tt.serverResponse == http.StatusTooManyRequests {
					w.Header().Set("Retry-After", "2")
					w.Write([]byte("Rate limit exceeded"))
				} else if tt.serverResponse == http.StatusBadRequest {
					w.Write([]byte(`{"error": {"code": "InvalidWebhookUrl", "message": "Webhook URL has expired"}}`))
				} else {
					w.Write([]byte("1"))
				}
			}))
			defer server.Close()

			// Update webhook URL to point to our test server
			tt.jobSpec.WebhookURL = server.URL

			// Create the Teams webhook service (this will fail until implemented)
			webhookService := NewTeamsWebhookService()

			// Attempt to send notification - this MUST fail until implementation
			ctx := context.Background()
			err := webhookService.SendNotification(ctx, tt.jobSpec)

			if tt.expectError != "" {
				assert.Error(t, err, "Expected error but got none")
				assert.Contains(t, strings.ToLower(err.Error()), strings.ToLower(tt.expectError), "Error message mismatch")
				return
			}

			require.NoError(t, err, "SendNotification should succeed")

			// Validate request headers
			assert.Equal(t, "application/json", receivedHeaders.Get("Content-Type"), "Content-Type header mismatch")
			assert.Contains(t, receivedHeaders.Get("User-Agent"), "joblin-cli", "User-Agent header should contain joblin-cli")

			// Validate the received payload structure
			assert.Equal(t, tt.expectedMessageCard.Type, receivedPayload.Type, "MessageCard type mismatch")
			assert.Equal(t, tt.expectedMessageCard.Context, receivedPayload.Context, "MessageCard context mismatch")
			assert.Equal(t, tt.expectedMessageCard.ThemeColor, receivedPayload.ThemeColor, "MessageCard theme color mismatch")
			assert.Equal(t, tt.expectedMessageCard.Summary, receivedPayload.Summary, "MessageCard summary mismatch")

			// Validate sections
			require.Len(t, receivedPayload.Sections, len(tt.expectedMessageCard.Sections), "Number of sections mismatch")

			for i, expectedSection := range tt.expectedMessageCard.Sections {
				actualSection := receivedPayload.Sections[i]
				assert.Equal(t, expectedSection.ActivityTitle, actualSection.ActivityTitle, "Section %d title mismatch", i)
				assert.Equal(t, expectedSection.ActivitySubtitle, actualSection.ActivitySubtitle, "Section %d subtitle mismatch", i)

				// Validate facts if present
				if len(expectedSection.Facts) > 0 {
					require.Len(t, actualSection.Facts, len(expectedSection.Facts), "Section %d facts count mismatch", i)
					for j, expectedFact := range expectedSection.Facts {
						actualFact := actualSection.Facts[j]
						assert.Equal(t, expectedFact.Name, actualFact.Name, "Section %d fact %d name mismatch", i, j)
						assert.Equal(t, expectedFact.Value, actualFact.Value, "Section %d fact %d value mismatch", i, j)
					}
				}

				// Validate text content if present
				if expectedSection.Text != "" {
					assert.Equal(t, expectedSection.Text, actualSection.Text, "Section %d text mismatch", i)
				}
			}
		})
	}
}

// TestTeamsWebhookRetryLogic tests the retry mechanism for webhook delivery
// This test MUST FAIL until the Teams webhook retry logic is implemented
func TestTeamsWebhookRetryLogic(t *testing.T) {
	tests := []struct {
		name                string
		serverResponses     []int           // Sequence of HTTP status codes
		serverDelays        []time.Duration // Delays for each response
		expectedRetryCount  int
		expectFinalSuccess  bool
		expectError         string
	}{
		{
			name:            "success_on_first_attempt",
			serverResponses: []int{http.StatusOK},
			expectedRetryCount: 0,
			expectFinalSuccess: true,
		},
		{
			name:            "success_after_rate_limit",
			serverResponses: []int{http.StatusTooManyRequests, http.StatusOK},
			serverDelays:    []time.Duration{0, 0},
			expectedRetryCount: 1,
			expectFinalSuccess: true,
		},
		{
			name:            "success_after_server_error",
			serverResponses: []int{http.StatusInternalServerError, http.StatusBadGateway, http.StatusOK},
			expectedRetryCount: 2,
			expectFinalSuccess: true,
		},
		{
			name:            "permanent_failure_bad_request",
			serverResponses: []int{http.StatusBadRequest},
			expectedRetryCount: 0,
			expectFinalSuccess: false,
			expectError:     "webhook URL invalid",
		},
		{
			name:            "max_retries_exceeded",
			serverResponses: []int{http.StatusTooManyRequests, http.StatusTooManyRequests, http.StatusTooManyRequests, http.StatusTooManyRequests, http.StatusTooManyRequests, http.StatusTooManyRequests},
			expectedRetryCount: 5, // Should stop at max retries
			expectFinalSuccess: false,
			expectError:     "max retry attempts exceeded",
		},
		{
			name:            "exponential_backoff_timing",
			serverResponses: []int{http.StatusTooManyRequests, http.StatusTooManyRequests, http.StatusOK},
			serverDelays:    []time.Duration{0, 0, 0},
			expectedRetryCount: 2,
			expectFinalSuccess: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			attemptCount := 0
			retryTimes := []time.Time{}

			// Create mock server that responds according to test case
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				retryTimes = append(retryTimes, time.Now())

				// Apply delay if specified
				if attemptCount < len(tt.serverDelays) && tt.serverDelays[attemptCount] > 0 {
					time.Sleep(tt.serverDelays[attemptCount])
				}

				// Send appropriate response
				if attemptCount < len(tt.serverResponses) {
					statusCode := tt.serverResponses[attemptCount]
					w.WriteHeader(statusCode)

					switch statusCode {
					case http.StatusOK:
						w.Write([]byte("1"))
					case http.StatusTooManyRequests:
						w.Header().Set("Retry-After", "1")
						w.Write([]byte("Rate limit exceeded"))
					case http.StatusBadRequest:
						w.Write([]byte(`{"error": {"code": "InvalidWebhookUrl"}}`))
					default:
						w.Write([]byte("Server Error"))
					}
				} else {
					// Default to OK if we've exhausted the response list
					w.WriteHeader(http.StatusOK)
					w.Write([]byte("1"))
				}

				attemptCount++
			}))
			defer server.Close()

			// Create test job notification
			jobNotification := JobNotification{
				JobID:           testJobID1,
				Name:            "retry-test-job",
				ScriptFilename:  testScriptPy,
				Status:          "completed",
				Duration:        "1m",
				KubernetesContext: testCluster,
				Namespace:       "default",
				ExitCode:        0,
				CompletedAt:     time.Now(),
				WebhookURL:      server.URL,
			}

			// Create the Teams webhook service with retry configuration
			webhookService := NewTeamsWebhookServiceWithRetry(RetryConfig{
				MaxAttempts:   5,
				BaseDelay:     100 * time.Millisecond,
				MaxDelay:      5 * time.Second,
				Multiplier:    2.0,
				JitterPercent: 10.0,
			})

			// Attempt to send notification
			startTime := time.Now()
			ctx := context.Background()
			err := webhookService.SendNotification(ctx, jobNotification)

			// Validate results
			if tt.expectFinalSuccess {
				assert.NoError(t, err, "Expected success but got error")
			} else {
				assert.Error(t, err, "Expected error but got success")
				if tt.expectError != "" {
					assert.Contains(t, strings.ToLower(err.Error()), strings.ToLower(tt.expectError), "Error message mismatch")
				}
			}

			// Validate retry count
			actualRetryCount := attemptCount - 1 // First attempt is not a retry
			assert.Equal(t, tt.expectedRetryCount, actualRetryCount, "Retry count mismatch")

			// Validate exponential backoff timing (if multiple attempts)
			if len(retryTimes) > 1 {
				for i := 1; i < len(retryTimes); i++ {
					timeDiff := retryTimes[i].Sub(retryTimes[i-1])
					expectedMinDelay := time.Duration(float64(100*time.Millisecond) * math.Pow(2, float64(i-1))) // 2^(attempt-1)

					// Allow for some timing variance in tests
					assert.GreaterOrEqual(t, timeDiff, expectedMinDelay/2, "Retry delay too short for attempt %d", i)
					assert.Less(t, timeDiff, 10*time.Second, "Retry delay too long for attempt %d", i)
				}
			}

			// Total operation should complete within reasonable time
			totalDuration := time.Since(startTime)
			assert.Less(t, totalDuration, 30*time.Second, "Operation took too long")
		})
	}
}

// TestTeamsWebhookMessageFormatting tests message formatting edge cases
// This test MUST FAIL until the Teams webhook message formatting is implemented
func TestTeamsWebhookMessageFormatting(t *testing.T) {
	tests := []struct {
		name           string
		jobSpec        JobNotification
		validateFields func(t *testing.T, card TeamsMessageCard)
	}{
		{
			name: "long_error_message_truncation",
			jobSpec: JobNotification{
				JobID:          testJobID1,
				Name:           "error-job",
				ScriptFilename: "error_script.py",
				Status:         "failed",
				ErrorMessage:   strings.Repeat("Very long error message ", 20), // >200 chars
				WebhookURL:     testWebhookURL,
			},
			validateFields: func(t *testing.T, card TeamsMessageCard) {
				errorSummary := ""
				for _, section := range card.Sections {
					for _, fact := range section.Facts {
						if fact.Name == "Error Summary" {
							errorSummary = fact.Value
							break
						}
					}
				}
				assert.LessOrEqual(t, len(errorSummary), 200, "Error message should be truncated to 200 characters")
			},
		},
		{
			name: "many_log_lines_truncation",
			jobSpec: JobNotification{
				JobID:          testJobID2,
				Name:           "logs-job",
				ScriptFilename: "logs_script.py",
				Status:         "failed",
				LastLogLines:   make([]string, 20), // More than 10 lines
				WebhookURL:     testWebhookURL,
			},
			validateFields: func(t *testing.T, card TeamsMessageCard) {
				logSection := ""
				for _, section := range card.Sections {
					if section.Title == "Last Log Entries" {
						logSection = section.Text
						break
					}
				}
				logLines := strings.Split(strings.Trim(logSection, "```\n"), "\n")
				assert.LessOrEqual(t, len(logLines), 10, "Log entries should be limited to 10 lines")
			},
		},
		{
			name: "special_characters_in_script_name",
			jobSpec: JobNotification{
				JobID:          testJobID3,
				Name:           "special-job",
				ScriptFilename: "special_script_with_üñíçødé.py",
				Status:         "completed",
				WebhookURL:     testWebhookURL,
			},
			validateFields: func(t *testing.T, card TeamsMessageCard) {
				scriptName := ""
				for _, section := range card.Sections {
					for _, fact := range section.Facts {
						if fact.Name == "Script" {
							scriptName = fact.Value
							break
						}
					}
				}
				assert.Equal(t, "special_script_with_üñíçødé.py", scriptName, "Script name should preserve Unicode characters")
			},
		},
		{
			name: "message_size_limit",
			jobSpec: JobNotification{
				JobID:          "12345678-1234-4234-8234-123456789015",
				Name:           strings.Repeat("very-long-job-name-", 100), // Very long name
				ScriptFilename: testScriptPy,
				Status:         "completed",
				WebhookURL:     testWebhookURL,
			},
			validateFields: func(t *testing.T, card TeamsMessageCard) {
				// Serialize the card to check total size
				cardJSON, err := json.Marshal(card)
				require.NoError(t, err, "Failed to marshal card to JSON")
				assert.LessOrEqual(t, len(cardJSON), 28*1024, "Message should be under 28KB limit")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var receivedCard TeamsMessageCard

			// Create mock server
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				body, err := io.ReadAll(r.Body)
				require.NoError(t, err, "Failed to read request body")

				err = json.Unmarshal(body, &receivedCard)
				require.NoError(t, err, "Failed to parse request JSON")

				w.WriteHeader(http.StatusOK)
				w.Write([]byte("1"))
			}))
			defer server.Close()

			tt.jobSpec.WebhookURL = server.URL

			// Create the Teams webhook service
			webhookService := NewTeamsWebhookService()

			// Send notification
			ctx := context.Background()
			err := webhookService.SendNotification(ctx, tt.jobSpec)
			require.NoError(t, err, "SendNotification should succeed")

			// Run custom validation
			tt.validateFields(t, receivedCard)
		})
	}
}

// Helper types and interfaces

// JobNotification represents a job notification for Teams webhook
type JobNotification struct {
	JobID             string
	Name              string
	ScriptFilename    string
	Status            string // completed, failed, terminated
	Duration          string
	KubernetesContext string
	Namespace         string
	ExitCode          int
	CompletedAt       time.Time
	TerminatedAt      time.Time
	TerminatedBy      string
	ErrorMessage      string
	LastLogLines      []string
	WebhookURL        string
}

// TeamsMessageCard represents a Microsoft Teams message card
type TeamsMessageCard struct {
	Type         string           `json:"@type"`
	Context      string           `json:"@context"`
	ThemeColor   string           `json:"themeColor"`
	Summary      string           `json:"summary"`
	Sections     []MessageSection `json:"sections"`
	PotentialAction []MessageAction `json:"potentialAction,omitempty"`
}

// MessageSection represents a section in a Teams message card
type MessageSection struct {
	ActivityTitle    string        `json:"activityTitle,omitempty"`
	ActivitySubtitle string        `json:"activitySubtitle,omitempty"`
	ActivityImage    string        `json:"activityImage,omitempty"`
	Facts           []MessageFact  `json:"facts,omitempty"`
	Title           string         `json:"title,omitempty"`
	Text            string         `json:"text,omitempty"`
	Markdown        bool           `json:"markdown,omitempty"`
}

// MessageFact represents a fact in a Teams message section
type MessageFact struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// MessageAction represents an action in a Teams message card
type MessageAction struct {
	Type    string          `json:"@type"`
	Name    string          `json:"name"`
	Targets []MessageTarget `json:"targets"`
}

// MessageTarget represents a target for a Teams message action
type MessageTarget struct {
	OS  string `json:"os"`
	URI string `json:"uri"`
}

// RetryConfig represents retry configuration for webhook delivery
type RetryConfig struct {
	MaxAttempts   int
	BaseDelay     time.Duration
	MaxDelay      time.Duration
	Multiplier    float64
	JitterPercent float64
}

// TeamsWebhookService interface - this will be implemented
type TeamsWebhookService interface {
	SendNotification(ctx context.Context, notification JobNotification) error
}

// NewTeamsWebhookService creates a new Teams webhook service - this will fail until implemented
func NewTeamsWebhookService() TeamsWebhookService {
	// This should return the actual implementation
	// For now, this will cause a compile error, which is expected
	panic("TeamsWebhookService not implemented - this test should fail until T024 is complete")
}

// NewTeamsWebhookServiceWithRetry creates a new Teams webhook service with retry config
func NewTeamsWebhookServiceWithRetry(config RetryConfig) TeamsWebhookService {
	// This should return the actual implementation with retry configuration
	// For now, this will cause a compile error, which is expected
	panic("TeamsWebhookService with retry not implemented - this test should fail until T024 is complete")
}