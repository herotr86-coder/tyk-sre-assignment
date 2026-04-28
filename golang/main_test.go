package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/version"
	disco "k8s.io/client-go/discovery/fake"
	"k8s.io/client-go/kubernetes/fake"
)

func TestGetKubernetesVersion(t *testing.T) {
	okClientset := fake.NewSimpleClientset()
	okClientset.Discovery().(*disco.FakeDiscovery).FakedServerVersion = &version.Info{GitVersion: "1.25.0-fake"}

	okVer, err := getKubernetesVersion(okClientset)
	assert.NoError(t, err)
	assert.Equal(t, "1.25.0-fake", okVer)

	badClientset := fake.NewSimpleClientset()
	badClientset.Discovery().(*disco.FakeDiscovery).FakedServerVersion = &version.Info{}

	badVer, err := getKubernetesVersion(badClientset)
	assert.NoError(t, err)
	assert.Equal(t, "", badVer)
}

func TestHealthHandler(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()

	healthHandler(rec, req)
	res := rec.Result()

	assert.Equal(t, http.StatusOK, res.StatusCode)

	defer func(Body io.ReadCloser) {
		assert.NoError(t, Body.Close())
	}(res.Body)
	resp, err := io.ReadAll(res.Body)

	assert.NoError(t, err)
	assert.Equal(t, "ok", string(resp))
}

func TestAPIHealthHandler(t *testing.T) {
	fakeCS := fake.NewSimpleClientset()
	fakeCS.Discovery().(*disco.FakeDiscovery).FakedServerVersion = &version.Info{GitVersion: "1.25.0-fake"}
	clientset = fakeCS

	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	rec := httptest.NewRecorder()
	apiHealthHandler(rec, req)

	assert.Equal(t, http.StatusOK, rec.Result().StatusCode)

	var body map[string]interface{}
	json.NewDecoder(rec.Body).Decode(&body)
	assert.Equal(t, "healthy", body["status"])
}

func TestDeploymentsHandler_AllHealthy(t *testing.T) {
	replicas := int32(2)
	fakeCS := fake.NewSimpleClientset(&appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "my-app", Namespace: "default"},
		Spec:       appsv1.DeploymentSpec{Replicas: &replicas},
		Status:     appsv1.DeploymentStatus{ReadyReplicas: 2},
	})
	clientset = fakeCS

	req := httptest.NewRequest(http.MethodGet, "/deployments", nil)
	rec := httptest.NewRecorder()
	deploymentsHandler(rec, req)

	assert.Equal(t, http.StatusOK, rec.Result().StatusCode)

	var body map[string]interface{}
	json.NewDecoder(rec.Body).Decode(&body)
	assert.Equal(t, float64(0), body["unhealthyDeployments"])
}

func TestDeploymentsHandler_Unhealthy(t *testing.T) {
	replicas := int32(3)
	fakeCS := fake.NewSimpleClientset(&appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "broken-app", Namespace: "default"},
		Spec:       appsv1.DeploymentSpec{Replicas: &replicas},
		Status:     appsv1.DeploymentStatus{ReadyReplicas: 1},
	})
	clientset = fakeCS

	req := httptest.NewRequest(http.MethodGet, "/deployments", nil)
	rec := httptest.NewRecorder()
	deploymentsHandler(rec, req)

	assert.Equal(t, http.StatusMultiStatus, rec.Result().StatusCode)

	var body map[string]interface{}
	json.NewDecoder(rec.Body).Decode(&body)
	assert.Equal(t, float64(1), body["unhealthyDeployments"])
}

func TestNamespaceDeploymentsHandler(t *testing.T) {
	replicas := int32(1)
	fakeCS := fake.NewSimpleClientset(&appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "svc", Namespace: "production"},
		Spec:       appsv1.DeploymentSpec{Replicas: &replicas},
		Status:     appsv1.DeploymentStatus{ReadyReplicas: 1},
	})
	clientset = fakeCS

	req := httptest.NewRequest(http.MethodGet, "/deployments/production", nil)
	rec := httptest.NewRecorder()
	namespaceDeploymentsHandler(rec, req)

	assert.Equal(t, http.StatusOK, rec.Result().StatusCode)

	var results []DeploymentStatus
	json.NewDecoder(rec.Body).Decode(&results)
	assert.Len(t, results, 1)
	assert.True(t, results[0].Healthy)
}

func TestNetworkPolicyHandler_Block(t *testing.T) {
	fakeCS := fake.NewSimpleClientset()
	clientset = fakeCS

	body := `{"namespace":"default","labelSelector":"app=web","block":true,"policyName":"test-block"}`
	req := httptest.NewRequest(http.MethodPost, "/network-policies", strings.NewReader(body))
	rec := httptest.NewRecorder()
	networkPolicyHandler(rec, req)

	assert.Equal(t, http.StatusCreated, rec.Result().StatusCode)

	// Verify the policy actually exists in the fake cluster
	policy, err := fakeCS.NetworkingV1().NetworkPolicies("default").Get(
		context.Background(), "test-block", metav1.GetOptions{},
	)
	assert.NoError(t, err)
	assert.Equal(t, "test-block", policy.Name)
}

func TestNetworkPolicyHandler_Unblock(t *testing.T) {
	// Pre-create a policy, then delete it
	fakeCS := fake.NewSimpleClientset()
	clientset = fakeCS

	// Block first
	blockBody := `{"namespace":"default","labelSelector":"app=web","block":true,"policyName":"test-block"}`
	req := httptest.NewRequest(http.MethodPost, "/network-policies", strings.NewReader(blockBody))
	rec := httptest.NewRecorder()
	networkPolicyHandler(rec, req)
	assert.Equal(t, http.StatusCreated, rec.Result().StatusCode)

	// Then unblock
	unblockBody := `{"namespace":"default","labelSelector":"app=web","block":false,"policyName":"test-block"}`
	req2 := httptest.NewRequest(http.MethodPost, "/network-policies", strings.NewReader(unblockBody))
	rec2 := httptest.NewRecorder()
	networkPolicyHandler(rec2, req2)
	assert.Equal(t, http.StatusOK, rec2.Result().StatusCode)

	// Confirm it's gone
	_, err := fakeCS.NetworkingV1().NetworkPolicies("default").Get(
		context.Background(), "test-block", metav1.GetOptions{},
	)
	assert.Error(t, err)
}

func TestNetworkPolicyHandler_WrongMethod(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/network-policies", nil)
	rec := httptest.NewRecorder()
	networkPolicyHandler(rec, req)
	assert.Equal(t, http.StatusMethodNotAllowed, rec.Result().StatusCode)
}

// suppress unused import
var _ = corev1.Pod{}