# Kubernetes API Contract: Joblin

## Job Resource Template

### Standard Job Manifest
Joblin creates Kubernetes Jobs with the following standardized structure:

```yaml
apiVersion: batch/v1
kind: Job
metadata:
  name: "joblin-{job-id-short}"  # First 8 chars of UUID
  namespace: "{user-namespace}"
  labels:
    app.kubernetes.io/name: "joblin"
    app.kubernetes.io/component: "python-job"
    app.kubernetes.io/managed-by: "joblin-cli"
    joblin.io/job-id: "{full-uuid}"
    joblin.io/user: "{current-user}"
    joblin.io/created-by: "joblin-v{version}"
  annotations:
    joblin.io/original-script: "{script-filename}"
    joblin.io/webhook-url: "{teams-webhook-url}"
    joblin.io/ttl-duration: "{ttl-string}"
spec:
  ttlSecondsAfterFinished: {ttl-seconds}
  backoffLimit: 3
  activeDeadlineSeconds: 3600  # 1 hour default timeout
  template:
    metadata:
      labels:
        app.kubernetes.io/name: "joblin"
        app.kubernetes.io/component: "python-job"
        joblin.io/job-id: "{full-uuid}"
    spec:
      restartPolicy: Never
      containers:
      - name: python-script
        image: "python:3.11-slim"
        command: ["/bin/bash", "-c"]
        args:
        - |
          pip install --no-cache-dir {dependencies}
          python /app/script.py
        resources:
          limits:
            cpu: "{cpu-limit}"
            memory: "{memory-limit}"
            ephemeral-storage: "{storage-limit}"
          requests:
            cpu: "{cpu-request}"  # 50% of limit
            memory: "{memory-request}"  # 50% of limit
        volumeMounts:
        - name: script-volume
          mountPath: /app
          readOnly: true
        env: {user-env-vars}
      volumes:
      - name: script-volume
        configMap:
          name: "joblin-script-{job-id-short}"
          defaultMode: 0755
```

### ConfigMap for Script Content
```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: "joblin-script-{job-id-short}"
  namespace: "{user-namespace}"
  labels:
    app.kubernetes.io/name: "joblin"
    app.kubernetes.io/component: "script-storage"
    joblin.io/job-id: "{full-uuid}"
  ownerReferences:
  - apiVersion: batch/v1
    kind: Job
    name: "joblin-{job-id-short}"
    uid: "{job-uid}"
    controller: true
data:
  script.py: |
    {script-content}
  requirements.txt: |
    {dependencies}
```

## API Operations

### Job Creation
**Operation**: Create Job and ConfigMap resources
**API Calls**:
1. `POST /api/v1/namespaces/{namespace}/configmaps`
2. `POST /apis/batch/v1/namespaces/{namespace}/jobs`

**Error Handling**:
- Resource quota exceeded: Provide clear error with limits
- Invalid namespace: Suggest available namespaces
- RBAC permissions: Guide user to required permissions

### Job Status Monitoring
**Operation**: Monitor job lifecycle and conditions
**API Calls**:
1. `GET /apis/batch/v1/namespaces/{namespace}/jobs/{job-name}`
2. `GET /api/v1/namespaces/{namespace}/pods?labelSelector=job-name={job-name}`

**Status Mapping**:
```go
// Kubernetes Job Conditions → Joblin Status
switch {
case job.Status.Succeeded > 0:
    return "completed"
case job.Status.Failed > 0:
    return "failed"
case job.Status.Active > 0:
    return "running"
default:
    return "pending"
}
```

### Log Retrieval
**Operation**: Stream logs from job pods
**API Calls**:
1. `GET /api/v1/namespaces/{namespace}/pods?labelSelector=job-name={job-name}`
2. `GET /api/v1/namespaces/{namespace}/pods/{pod-name}/log`

**Log Parameters**:
- `follow=true`: Stream logs in real-time
- `tailLines={n}`: Limit number of lines
- `sinceTime={rfc3339}`: Logs since timestamp
- `timestamps=true`: Include timestamps

### Job Termination
**Operation**: Delete job and associated resources
**API Calls**:
1. `DELETE /apis/batch/v1/namespaces/{namespace}/jobs/{job-name}?propagationPolicy=Foreground`
2. ConfigMap is deleted automatically via ownerReference

**Termination Options**:
- `gracePeriodSeconds=30`: Standard graceful termination
- `gracePeriodSeconds=0`: Force termination

## RBAC Requirements

### Minimum Permissions
Joblin requires the following Kubernetes permissions:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: joblin-user
rules:
# Job management
- apiGroups: ["batch"]
  resources: ["jobs"]
  verbs: ["create", "get", "list", "watch", "delete"]
# ConfigMap for scripts
- apiGroups: [""]
  resources: ["configmaps"]
  verbs: ["create", "get", "delete"]
# Pod log access
- apiGroups: [""]
  resources: ["pods", "pods/log"]
  verbs: ["get", "list", "watch"]
# Namespace access
- apiGroups: [""]
  resources: ["namespaces"]
  verbs: ["get", "list"]
```

### Namespace-Scoped Alternative
```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  namespace: {user-namespace}
  name: joblin-namespace-user
rules:
- apiGroups: ["batch"]
  resources: ["jobs"]
  verbs: ["create", "get", "list", "watch", "delete"]
- apiGroups: [""]
  resources: ["configmaps", "pods", "pods/log"]
  verbs: ["create", "get", "list", "watch", "delete"]
```

## Resource Naming Conventions

### Job Names
- Format: `joblin-{job-id-first-8-chars}`
- Example: `joblin-a1b2c3d4`
- Ensures uniqueness and DNS-1123 compliance

### ConfigMap Names
- Format: `joblin-script-{job-id-first-8-chars}`
- Example: `joblin-script-a1b2c3d4`
- Linked to Job via ownerReference

### Label Standards
- `app.kubernetes.io/name`: Always "joblin"
- `app.kubernetes.io/component`: "python-job" or "script-storage"
- `app.kubernetes.io/managed-by`: "joblin-cli"
- `joblin.io/job-id`: Full UUID for cross-referencing
- `joblin.io/user`: Current user identifier
- User-defined labels: Prefixed with `user.joblin.io/`

## Error Responses

### Standard Kubernetes Error Format
```json
{
  "kind": "Status",
  "apiVersion": "v1",
  "metadata": {},
  "status": "Failure",
  "message": "jobs.batch \"joblin-12345678\" already exists",
  "reason": "AlreadyExists",
  "code": 409
}
```

### Common Error Scenarios
1. **Resource Quota Exceeded**
   - HTTP 403: Insufficient quota
   - Solution: Increase quota or reduce resource requests

2. **Invalid Job Spec**
   - HTTP 422: Validation failed
   - Common: Invalid resource quantities, container image issues

3. **RBAC Permissions**
   - HTTP 403: User lacks required permissions
   - Solution: Apply proper Role/ClusterRole bindings

4. **Namespace Not Found**
   - HTTP 404: Namespace doesn't exist
   - Solution: Create namespace or use existing one

## Monitoring and Observability

### Job Events
Monitor Kubernetes events for job lifecycle:
```go
watch.Interface, err := clientset.CoreV1().Events(namespace).Watch(context.TODO(), metav1.ListOptions{
    FieldSelector: fmt.Sprintf("involvedObject.name=%s", jobName),
})
```

### Success Policy (Kubernetes 1.28+)
For advanced job completion criteria:
```yaml
spec:
  successPolicy:
    rules:
    - succeededIndexes: "0-2"  # Early exit when first 3 pods succeed
```

### Resource Monitoring
Track resource usage via metrics API:
- CPU/Memory utilization
- Pod restart counts
- Job duration metrics

This Kubernetes API contract ensures consistent interaction with the cluster while following best practices for resource management, security, and observability.