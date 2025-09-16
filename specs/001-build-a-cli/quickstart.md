# Quickstart Guide: Joblin CLI

## Prerequisites
- Kubernetes cluster access with kubeconfig configured
- Go 1.21+ installed for building from source
- Valid Teams webhook URL (optional, for notifications)

## Installation

### Option 1: Download Binary (Recommended)
```bash
# Download latest release for your platform
curl -LO https://github.com/berkunal/joblin/releases/latest/download/joblin-linux-amd64
chmod +x joblin-linux-amd64
sudo mv joblin-linux-amd64 /usr/local/bin/joblin

# Verify installation
joblin --version
```

### Option 2: Build from Source
```bash
git clone https://github.com/berkunal/joblin.git
cd joblin
go build -o joblin ./cmd/joblin
sudo mv joblin /usr/local/bin/joblin
```

## Initial Setup

### 1. Initialize Configuration
```bash
# Create default configuration
joblin config init

# Set default Teams webhook (optional)
joblin config set webhook.url "https://your-teams-webhook-url"

# Verify configuration
joblin config show
```

### 2. Test Kubernetes Connectivity
```bash
# Check cluster access
kubectl cluster-info

# Verify joblin can access cluster
joblin list
```

## Basic Usage Examples

### Example 1: Simple Python Script
Create a simple Python script:
```bash
cat > hello.py << 'EOF'
#!/usr/bin/env python3
print("Hello from Kubernetes!")
print("This job is running successfully.")
EOF
```

Deploy and monitor the job:
```bash
# Deploy the script
joblin deploy hello.py --name hello-world

# Check job status
joblin status

# View logs
joblin logs <job-id>

# Wait for completion (optional)
joblin deploy hello.py --name hello-wait --wait
```

### Example 2: Python Script with Dependencies
Create a script with external dependencies:
```bash
cat > requirements.txt << 'EOF'
requests==2.31.0
pandas==2.1.0
EOF

cat > data_processor.py << 'EOF'
#!/usr/bin/env python3
import requests
import pandas as pd
import json

# Fetch sample data
response = requests.get('https://jsonplaceholder.typicode.com/posts')
data = response.json()

# Process with pandas
df = pd.DataFrame(data)
print(f"Processed {len(df)} records")
print(f"Unique users: {df['userId'].nunique()}")

# Output summary
summary = {
    "total_posts": len(df),
    "unique_users": df['userId'].nunique(),
    "avg_title_length": df['title'].str.len().mean()
}
print(json.dumps(summary, indent=2))
EOF
```

Deploy with custom resources:
```bash
# Deploy with requirements and custom resources
joblin deploy data_processor.py \
  --name data-processing \
  --requirements requirements.txt \
  --cpu "200m" \
  --memory "256Mi" \
  --webhook "https://your-teams-webhook-url"

# Monitor with continuous watching
joblin status --watch
```

### Example 3: Long-Running Job with Monitoring
Create a long-running script:
```bash
cat > long_task.py << 'EOF'
#!/usr/bin/env python3
import time
import sys

print("Starting long-running task...")

for i in range(10):
    print(f"Progress: {i+1}/10 - Processing batch {i+1}")
    time.sleep(30)  # Simulate work

    if i == 5:
        print("Halfway point reached!")

print("Task completed successfully!")
EOF
```

Deploy and manage:
```bash
# Deploy long-running job
joblin deploy long_task.py \
  --name long-task \
  --ttl "2h" \
  --timeout "15m"

# Follow logs in real-time
joblin logs <job-id> --follow

# Terminate if needed (in another terminal)
joblin terminate <job-id>
```

## User Journey Test Scenarios

### Scenario 1: First-Time User Setup
**Test Steps**:
1. Install joblin CLI
2. Run `joblin config init`
3. Create simple hello.py script
4. Deploy with `joblin deploy hello.py`
5. Check status with `joblin status`
6. View logs with `joblin logs <job-id>`

**Expected Outcomes**:
- Configuration created in ~/.joblin/config.yaml
- Job deploys successfully to default namespace
- Status shows "completed" within 2 minutes
- Logs show "Hello from Kubernetes!" message

### Scenario 2: Developer Workflow
**Test Steps**:
1. Create Python script with dependencies
2. Deploy with custom resource limits
3. Monitor job progress
4. Receive Teams notification on completion
5. Clean up old jobs

**Expected Outcomes**:
- Job runs with specified resource limits
- Dependencies install correctly
- Teams webhook delivers notification
- Old jobs are cleaned up automatically

### Scenario 3: Error Handling
**Test Steps**:
1. Deploy script with syntax error
2. Deploy with invalid resource limits
3. Deploy to non-existent namespace
4. Deploy with malformed webhook URL

**Expected Outcomes**:
- Clear error messages for each failure
- No orphaned Kubernetes resources
- Proper exit codes for scripting
- Helpful suggestions for resolution

### Scenario 4: Multi-Job Management
**Test Steps**:
1. Deploy multiple jobs simultaneously
2. List all jobs with filtering
3. Terminate specific jobs
4. View logs from multiple jobs
5. Clean up completed jobs

**Expected Outcomes**:
- All jobs tracked independently
- Filtering works correctly
- Termination is immediate
- Log retrieval is accurate
- Cleanup removes only intended jobs

## Integration Test Commands

### Quick Validation
```bash
# Run full integration test suite
make test-integration

# Test specific scenarios
make test-scenario-1  # First-time user
make test-scenario-2  # Developer workflow
make test-scenario-3  # Error handling
make test-scenario-4  # Multi-job management
```

### Manual Validation Checklist
- [ ] CLI installs without errors
- [ ] Configuration initializes correctly
- [ ] Simple script deploys and completes
- [ ] Scripts with dependencies work
- [ ] Resource limits are enforced
- [ ] Logs are retrievable
- [ ] Jobs can be terminated
- [ ] Teams notifications are delivered
- [ ] Cleanup removes old jobs
- [ ] Error messages are helpful

## Advanced Usage

### Environment Variables
```bash
# Deploy with environment variables
joblin deploy script.py \
  --env "API_KEY=secret123" \
  --env "ENVIRONMENT=production"
```

### Custom Labels
```bash
# Deploy with labels for organization
joblin deploy script.py \
  --labels "team=data-science" \
  --labels "project=analysis"

# Filter jobs by labels
joblin list --labels "team=data-science"
```

### Multiple Clusters
```bash
# Deploy to specific cluster context
joblin deploy script.py --context production-cluster

# Deploy to specific namespace
joblin deploy script.py --namespace my-team
```

### Batch Operations
```bash
# Deploy multiple scripts
for script in *.py; do
  joblin deploy "$script" --name "batch-$(basename $script .py)"
done

# Terminate all running jobs
joblin list --status running --json | jq -r '.jobs[].jobId' | xargs -I {} joblin terminate {}
```

## Troubleshooting

### Common Issues
1. **Job stays in Pending state**
   - Check resource quotas: `kubectl describe resourcequota`
   - Verify node capacity: `kubectl describe nodes`

2. **Authentication errors**
   - Verify kubeconfig: `kubectl cluster-info`
   - Check RBAC permissions: `kubectl auth can-i create jobs`

3. **Teams notifications not working**
   - Test webhook URL manually
   - Check webhook URL expiration (2025 requirement)

4. **Script dependencies fail**
   - Verify requirements.txt format
   - Check network policies for pip access

### Getting Help
```bash
# Show help for any command
joblin deploy --help
joblin logs --help

# Show configuration
joblin config show

# Enable verbose logging
joblin --verbose deploy script.py
```

This quickstart guide provides a complete user journey from installation to advanced usage, serving as both documentation and executable test scenarios for validating the Joblin CLI functionality.