# Teams Webhook Contract: Joblin

## Webhook Message Format

### Success Notification
```json
{
  "@type": "MessageCard",
  "@context": "http://schema.org/extensions",
  "themeColor": "00FF00",
  "summary": "Joblin Job Completed Successfully",
  "sections": [
    {
      "activityTitle": "✅ Job Completed Successfully",
      "activitySubtitle": "Job: {job-name}",
      "activityImage": "https://via.placeholder.com/64x64/00FF00/FFFFFF?text=✓",
      "facts": [
        {
          "name": "Job ID",
          "value": "{job-id}"
        },
        {
          "name": "Script",
          "value": "{script-filename}"
        },
        {
          "name": "Duration",
          "value": "{duration-human-readable}"
        },
        {
          "name": "Cluster",
          "value": "{kubernetes-context}"
        },
        {
          "name": "Namespace",
          "value": "{kubernetes-namespace}"
        },
        {
          "name": "Exit Code",
          "value": "0"
        },
        {
          "name": "Completed At",
          "value": "{completion-timestamp-rfc3339}"
        }
      ],
      "markdown": true
    }
  ],
  "potentialAction": [
    {
      "@type": "OpenUri",
      "name": "View Logs",
      "targets": [
        {
          "os": "default",
          "uri": "https://dashboard.example.com/jobs/{job-id}/logs"
        }
      ]
    }
  ]
}
```

### Failure Notification
```json
{
  "@type": "MessageCard",
  "@context": "http://schema.org/extensions",
  "themeColor": "FF0000",
  "summary": "Joblin Job Failed",
  "sections": [
    {
      "activityTitle": "❌ Job Failed",
      "activitySubtitle": "Job: {job-name}",
      "activityImage": "https://via.placeholder.com/64x64/FF0000/FFFFFF?text=✗",
      "facts": [
        {
          "name": "Job ID",
          "value": "{job-id}"
        },
        {
          "name": "Script",
          "value": "{script-filename}"
        },
        {
          "name": "Duration",
          "value": "{duration-human-readable}"
        },
        {
          "name": "Cluster",
          "value": "{kubernetes-context}"
        },
        {
          "name": "Namespace",
          "value": "{kubernetes-namespace}"
        },
        {
          "name": "Exit Code",
          "value": "{exit-code}"
        },
        {
          "name": "Failed At",
          "value": "{failure-timestamp-rfc3339}"
        },
        {
          "name": "Error Summary",
          "value": "{error-message-truncated-200-chars}"
        }
      ],
      "markdown": true
    },
    {
      "title": "Last Log Entries",
      "text": "```\n{last-10-log-lines}\n```",
      "markdown": true
    }
  ],
  "potentialAction": [
    {
      "@type": "OpenUri",
      "name": "View Full Logs",
      "targets": [
        {
          "os": "default",
          "uri": "https://dashboard.example.com/jobs/{job-id}/logs"
        }
      ]
    },
    {
      "@type": "OpenUri",
      "name": "Restart Job",
      "targets": [
        {
          "os": "default",
          "uri": "https://dashboard.example.com/jobs/{job-id}/restart"
        }
      ]
    }
  ]
}
```

### Termination Notification
```json
{
  "@type": "MessageCard",
  "@context": "http://schema.org/extensions",
  "themeColor": "FFA500",
  "summary": "Joblin Job Terminated",
  "sections": [
    {
      "activityTitle": "⚠️ Job Terminated by User",
      "activitySubtitle": "Job: {job-name}",
      "activityImage": "https://via.placeholder.com/64x64/FFA500/FFFFFF?text=⚠",
      "facts": [
        {
          "name": "Job ID",
          "value": "{job-id}"
        },
        {
          "name": "Script",
          "value": "{script-filename}"
        },
        {
          "name": "Runtime",
          "value": "{runtime-duration-human-readable}"
        },
        {
          "name": "Cluster",
          "value": "{kubernetes-context}"
        },
        {
          "name": "Namespace",
          "value": "{kubernetes-namespace}"
        },
        {
          "name": "Terminated At",
          "value": "{termination-timestamp-rfc3339}"
        },
        {
          "name": "Terminated By",
          "value": "{user-identifier}"
        }
      ],
      "markdown": true
    }
  ]
}
```

## HTTP Request Specification

### Request Headers
```http
POST {teams-webhook-url}
Content-Type: application/json
User-Agent: joblin-cli/{version}
Content-Length: {message-size}
```

### Rate Limiting Compliance
- **Limit**: 4 requests per second per webhook URL
- **Implementation**: Exponential backoff with jitter
- **Retry Strategy**: 5 attempts maximum
- **Backoff Formula**: `delay = base_delay * (2^attempt) + random_jitter`
- **Base Delay**: 1 second
- **Maximum Delay**: 60 seconds

### Message Size Constraints
- **Maximum Size**: 28 KB per message
- **Content Truncation**:
  - Error messages: 200 characters
  - Log entries: 10 lines maximum
  - Script names: 50 characters

### Response Handling

#### Success Response
```http
HTTP/1.1 200 OK
Content-Type: text/plain
Content-Length: 1

1
```

#### Rate Limit Response
```http
HTTP/1.1 429 Too Many Requests
Retry-After: 2

Rate limit exceeded
```

#### Webhook URL Expired (2025 Update)
```http
HTTP/1.1 400 Bad Request
Content-Type: application/json

{
  "error": {
    "code": "InvalidWebhookUrl",
    "message": "Webhook URL has expired. Please update to new connector format by December 31, 2024."
  }
}
```

## Error Handling and Retry Logic

### Retry Conditions
- HTTP 429 (Rate Limited): Retry with exponential backoff
- HTTP 5xx (Server Errors): Retry up to 5 times
- Network timeouts: Retry with increased timeout
- DNS resolution failures: Retry after 30 seconds

### Non-Retry Conditions
- HTTP 400 (Bad Request): Log error, do not retry
- HTTP 401/403 (Unauthorized): Log error, do not retry
- HTTP 404 (Not Found): Webhook URL invalid, do not retry
- Malformed JSON: Log error, do not retry

### Retry Implementation
```go
type RetryConfig struct {
    MaxAttempts   int           // 5
    BaseDelay     time.Duration // 1 second
    MaxDelay      time.Duration // 60 seconds
    Multiplier    float64       // 2.0
    JitterPercent float64       // 10%
}

func (c *RetryConfig) NextDelay(attempt int) time.Duration {
    delay := time.Duration(float64(c.BaseDelay) * math.Pow(c.Multiplier, float64(attempt)))
    if delay > c.MaxDelay {
        delay = c.MaxDelay
    }

    jitter := time.Duration(float64(delay) * c.JitterPercent / 100)
    jitterOffset := time.Duration(rand.Int63n(int64(jitter*2))) - jitter

    return delay + jitterOffset
}
```

## Security Considerations

### Webhook URL Protection
- Store webhook URLs encrypted in local configuration
- Redact URLs from logs and error messages
- Validate HTTPS protocol requirement
- Support webhook URL rotation

### Data Sensitivity
- **Include**: Job ID, script filename, exit codes, timestamps
- **Exclude**: Script content, environment variables, sensitive logs
- **Sanitize**: Error messages that might contain secrets

### Network Security
- **TLS Verification**: Enforce valid certificates
- **Timeout Configuration**: 30-second request timeout
- **Connection Pooling**: Reuse connections for efficiency
- **Circuit Breaker**: Disable webhooks after consecutive failures

## Monitoring and Observability

### Notification Metrics
Track the following metrics for webhook operations:
- Notification delivery success rate
- Average delivery latency
- Retry attempt distribution
- Rate limit encounter frequency
- Webhook URL validation failures

### Audit Logging
Log webhook operations with structured format:
```json
{
  "timestamp": "2025-09-15T10:35:00Z",
  "level": "info",
  "component": "webhook",
  "operation": "notification_sent",
  "job_id": "uuid-v4",
  "notification_type": "success",
  "webhook_id": "sha256-hash-of-url",
  "attempt": 1,
  "response_code": 200,
  "latency_ms": 245
}
```

## Configuration

### Webhook Configuration Structure
```yaml
webhooks:
  default_url: "https://teams.microsoft.com/l/webhook/..."
  timeout: "30s"
  retry:
    max_attempts: 5
    base_delay: "1s"
    max_delay: "60s"
  rate_limit:
    requests_per_second: 4
    burst: 1
```

### Per-Job Webhook Override
Allow job-specific webhook URLs via CLI flags:
```bash
joblin deploy script.py --webhook "https://teams.microsoft.com/l/webhook/..."
```

This Teams webhook contract ensures reliable notification delivery while respecting Microsoft Teams rate limits and security requirements, providing users with timely and informative job status updates.