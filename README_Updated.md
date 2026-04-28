markdown
# Tyk SRE Assignment - Extended Tool

This is an extended version of the Kubernetes SRE tool with additional features for deployment health monitoring, network policy management, containerization, and Helm deployment.

## Original Assignment

The original tool provided basic Kubernetes connectivity testing. This extension adds production-ready SRE capabilities.

## New Features Implemented

### As an SRE (Stories 1 & 2)

#### 1. Deployment Health Checker (`SRE Story 1`)
Monitor the health of all deployments across the cluster or within a specific namespace.

**Endpoints:**
- `GET /deployments` - Check all deployments across all namespaces
- `GET /deployments/{namespace}` - Check deployments in a specific namespace

**Response Example:**
```json
{
  "totalDeployments": 5,
  "unhealthyDeployments": 1,
  "deployments": [
    {
      "name": "my-app",
      "namespace": "default",
      "desiredPods": 3,
      "readyPods": 3,
      "healthy": true
    },
    {
      "name": "broken-app",
      "namespace": "production",
      "desiredPods": 2,
      "readyPods": 0,
      "healthy": false,
      "message": "expected 2 pods, 0 ready"
    }
  ]
}
2. Network Policy Manager (SRE Story 2)
Dynamically block or unblock network traffic between workloads using Kubernetes NetworkPolicies.

Endpoint:

POST /network-policies - Create or delete network policies

Request Body to BLOCK traffic:

json
{
  "namespace": "default",
  "labelSelector": "app=web",
  "block": true,
  "policyName": "block-web-traffic"
}
Request Body to UNBLOCK traffic:

json
{
  "namespace": "default",
  "labelSelector": "app=web",
  "block": false,
  "policyName": "block-web-traffic"
}
3. API Health Check with Latency (SRE Story 3)
Check Kubernetes API server connectivity and measure response latency.

Endpoint:

GET /api/health

Response Example:

json
{
  "status": "healthy",
  "latency_ms": 45
}
As an Application Developer (Story 3)
4. Docker Containerization
Multi-stage Dockerfile for efficient, secure container builds.

Build the image:


docker build -t tyk-sre-assignment:latest ./golang
Run the container:


docker run -p 8080:8080 tyk-sre-assignment:latest
5. Helm Chart for Kubernetes Deployment
Complete Helm chart with proper RBAC, probes, and resource management.

Install the chart:


helm install tyk-sre ./helm
Chart Structure:


helm/
├── Chart.yaml          # Chart metadata
├── values.yaml         # Configuration values
└── templates/
    ├── deployment.yaml        # Main application deployment
    ├── service.yaml           # ClusterIP service
    ├── serviceaccount.yaml    # Service account
    ├── clusterrole.yaml       # RBAC permissions
    ├── clusterrolebinding.yaml # RBAC binding
    └── _helpers.tpl           # Template helpers
RBAC Permissions Granted:

List/watch deployments across all namespaces

Create/delete network policies

Read deployment statuses

6. CI/CD Pipeline with GitHub Actions
Automated testing and container image publishing to GitHub Container Registry (GHCR).

Workflow Steps:

Run Go unit tests

Build Docker image

Push to GHCR with tags:

latest

Git SHA (for versioning)

Complete API Documentation
Method	Endpoint	Description
GET	/healthz	Basic liveness probe
GET	/api/health	Kubernetes API health with latency
GET	/deployments	All deployments across all namespaces
GET	/deployments/{namespace}	Deployments in specific namespace
POST	/network-policies	Block/unblock network traffic
Getting Started
Prerequisites
Go 1.19+

Docker (optional, for containerization)

Kubernetes cluster (for actual deployment)

Helm 3.0+ (for Helm deployment)

Local Development
Clone the repository:


git clone https://github.com/herotr86-coder/tyk-sre-assignment.git
cd tyk-sre-assignment
Build the binary:


cd golang
go mod tidy
go build
Run locally (requires Kubernetes cluster):


./tyk-sre-assignment --kubeconfig ~/.kube/config --address :8080
Run unit tests:


go test -v
Docker Deployment
Build the image:


docker build -t tyk-sre-assignment:latest ./golang
Run the container:


docker run -d -p 8080:8080 \
  -v ~/.kube/config:/root/.kube/config \
  tyk-sre-assignment:latest
Helm Deployment on Kubernetes
Install the chart:


helm install tyk-sre ./helm
Customize values:


# Create custom values file
cat > custom-values.yaml << EOF
replicaCount: 3
image:
  repository: ghcr.io/herotr86-coder/tyk-sre-assignment
  tag: latest
resources:
  limits:
    cpu: 500m
    memory: 256Mi
EOF

# Install with custom values
helm install tyk-sre ./helm -f custom-values.yaml
Upgrade the release:


helm upgrade tyk-sre ./helm
Uninstall:


helm uninstall tyk-sre
CI/CD Pipeline
The GitHub Actions workflow automatically:

Runs tests on every push to main branch

Builds Docker image only if tests pass

Pushes image to GitHub Container Registry

View the workflow: .github/workflows/build.yml

Image location: ghcr.io/herotr86-coder/tyk-sre-assignment:latest

Testing the Endpoints
Once the service is running:


# Health check
curl http://localhost:8080/healthz

# API health with latency
curl http://localhost:8080/api/health

# Get all deployments
curl http://localhost:8080/deployments

# Get deployments in default namespace
curl http://localhost:8080/deployments/default

# Block traffic for app=web labels
curl -X POST http://localhost:8080/network-policies \
  -H "Content-Type: application/json" \
  -d '{
    "namespace": "default",
    "labelSelector": "app=web",
    "block": true,
    "policyName": "block-web"
  }'

# Unblock traffic
curl -X POST http://localhost:8080/network-policies \
  -H "Content-Type: application/json" \
  -d '{
    "namespace": "default",
    "labelSelector": "app=web",
    "block": false,
    "policyName": "block-web"
  }'
Troubleshooting
Helm lint errors

# Validate chart structure
helm lint ./helm

# Render templates locally without cluster
helm template tyk-sre ./helm
Permission denied when pushing to GitHub
Ensure you're pushing to your fork, not the original repository:


git remote set-url origin git@github.com:YOUR_USERNAME/tyk-sre-assignment.git
Kubernetes connection issues
The tool requires a valid kubeconfig:


# Verify cluster connectivity
kubectl cluster-info

# Run with explicit kubeconfig
./tyk-sre-assignment --kubeconfig ~/.kube/config
Files Changed/Added

tyk-sre-assignment/
├── .github/workflows/build.yml     # NEW: CI/CD pipeline
├── golang/
│   ├── Dockerfile                   # NEW: Container definition
│   ├── main.go                      # MODIFIED: Added all new endpoints
│   └── main_test.go                 # MODIFIED: Added comprehensive tests
├── helm/                            # NEW: Complete Helm chart
│   ├── Chart.yaml
│   ├── values.yaml
│   └── templates/
│       ├── _helpers.tpl
│       ├── clusterrole.yaml
│       ├── clusterrolebinding.yaml
│       ├── deployment.yaml
│       ├── service.yaml
│       └── serviceaccount.yaml
└── README.md                        # MODIFIED: This documentation
Interview Discussion Points
Be prepared to discuss:

Why multi-stage Docker build? - Smaller image size, better security

Network policy design - Why deny-all ingress/egress for isolation

Helm chart decisions - RBAC minimal permissions, probe configurations

Testing strategy - Fake clientset vs real cluster testing

Error handling - HTTP status codes (207 Multi-Status for partial failures)

Production readiness - Resource limits, liveness/readiness probes

License
MPL-2.0 - See LICENSE.md

Pull Request
PR Link: https://github.com/TykTechnologies/tyk-sre-assignment/pull/5

Author: Venkata Sai (herotr86@gmail.com)