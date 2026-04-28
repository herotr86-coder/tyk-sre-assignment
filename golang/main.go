package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"strings"
	"time"

	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

var clientset kubernetes.Interface

type DeploymentStatus struct {
	Name        string `json:"name"`
	Namespace   string `json:"namespace"`
	DesiredPods int32  `json:"desiredPods"`
	ReadyPods   int32  `json:"readyPods"`
	Healthy     bool   `json:"healthy"`
	Message     string `json:"message,omitempty"`
}

func main() {
	kubeconfig := flag.String("kubeconfig", "", "path to kubeconfig, leave empty for in-cluster")
	listenAddr := flag.String("address", ":8080", "HTTP server listen address")
	flag.Parse()

	kConfig, err := clientcmd.BuildConfigFromFlags("", *kubeconfig)
	if err != nil {
		panic(err)
	}

	clientset, err = kubernetes.NewForConfig(kConfig)
	if err != nil {
		panic(err)
	}

	version, err := getKubernetesVersion(clientset)
	if err != nil {
		panic(err)
	}

	fmt.Printf("Connected to Kubernetes %s\n", version)

	if err := startServer(*listenAddr); err != nil {
		panic(err)
	}
}

func getKubernetesVersion(cs kubernetes.Interface) (string, error) {
	version, err := cs.Discovery().ServerVersion()
	if err != nil {
		return "", err
	}
	return version.String(), nil
}

func startServer(listenAddr string) error {
	http.HandleFunc("/healthz", healthHandler)
	http.HandleFunc("/api/health", apiHealthHandler)
	http.HandleFunc("/deployments", deploymentsHandler)
	http.HandleFunc("/deployments/", namespaceDeploymentsHandler)
	http.HandleFunc("/network-policies", networkPolicyHandler)

	fmt.Printf("Server listening on %s\n", listenAddr)
	fmt.Println("Endpoints:")
	fmt.Println("  GET  /healthz")
	fmt.Println("  GET  /api/health")
	fmt.Println("  GET  /deployments")
	fmt.Println("  GET  /deployments/{namespace}")
	fmt.Println("  POST /network-policies")

	return http.ListenAndServe(listenAddr, nil)
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write([]byte("ok")); err != nil {
		fmt.Println("failed writing to response")
	}
}

// apiHealthHandler - SRE Story 3: checks connectivity to the k8s API server
func apiHealthHandler(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	_, err := clientset.Discovery().ServerVersion()
	latency := time.Since(start).Milliseconds()

	w.Header().Set("Content-Type", "application/json")

	resp := map[string]interface{}{
		"latency_ms": latency,
	}
	if err != nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		resp["status"] = "unhealthy"
		resp["error"] = err.Error()
	} else {
		w.WriteHeader(http.StatusOK)
		resp["status"] = "healthy"
	}
	json.NewEncoder(w).Encode(resp)
}

// deploymentsHandler - SRE Story 1: checks all deployments across all namespaces
func deploymentsHandler(w http.ResponseWriter, r *http.Request) {
	deployments, err := clientset.AppsV1().Deployments("").List(context.Background(), metav1.ListOptions{})
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to list deployments: %v", err), http.StatusInternalServerError)
		return
	}

	var results []DeploymentStatus
	unhealthy := 0

	for _, d := range deployments.Items {
		desired := int32(0)
		if d.Spec.Replicas != nil {
			desired = *d.Spec.Replicas
		}
		ready := d.Status.ReadyReplicas
		healthy := ready == desired

		s := DeploymentStatus{
			Name:        d.Name,
			Namespace:   d.Namespace,
			DesiredPods: desired,
			ReadyPods:   ready,
			Healthy:     healthy,
		}
		if !healthy {
			unhealthy++
			s.Message = fmt.Sprintf("expected %d pods, %d ready", desired, ready)
		}
		results = append(results, s)
	}

	w.Header().Set("Content-Type", "application/json")
	if unhealthy > 0 {
		w.WriteHeader(http.StatusMultiStatus)
	}
	json.NewEncoder(w).Encode(map[string]interface{}{
		"totalDeployments":     len(results),
		"unhealthyDeployments": unhealthy,
		"deployments":          results,
	})
}

// namespaceDeploymentsHandler - SRE Story 1: scoped to a specific namespace
func namespaceDeploymentsHandler(w http.ResponseWriter, r *http.Request) {
	namespace := strings.TrimPrefix(r.URL.Path, "/deployments/")
	if namespace == "" {
		http.Error(w, "namespace required", http.StatusBadRequest)
		return
	}

	deployments, err := clientset.AppsV1().Deployments(namespace).List(context.Background(), metav1.ListOptions{})
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to list deployments: %v", err), http.StatusInternalServerError)
		return
	}

	var results []DeploymentStatus
	for _, d := range deployments.Items {
		desired := int32(0)
		if d.Spec.Replicas != nil {
			desired = *d.Spec.Replicas
		}
		ready := d.Status.ReadyReplicas
		healthy := ready == desired

		s := DeploymentStatus{
			Name:        d.Name,
			Namespace:   d.Namespace,
			DesiredPods: desired,
			ReadyPods:   ready,
			Healthy:     healthy,
		}
		if !healthy {
			s.Message = fmt.Sprintf("expected %d pods, %d ready", desired, ready)
		}
		results = append(results, s)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(results)
}

// networkPolicyHandler - SRE Story 2: block/unblock traffic between two workloads
func networkPolicyHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "only POST allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Namespace     string `json:"namespace"`
		LabelSelector string `json:"labelSelector"`
		Block         bool   `json:"block"`
		PolicyName    string `json:"policyName"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("invalid body: %v", err), http.StatusBadRequest)
		return
	}

	if req.PolicyName == "" {
		req.PolicyName = fmt.Sprintf("block-%s", req.LabelSelector)
	}

	w.Header().Set("Content-Type", "application/json")

	if req.Block {
		// Parse label selector into map
		labelMap := map[string]string{}
		for _, part := range strings.Split(req.LabelSelector, ",") {
			kv := strings.SplitN(strings.TrimSpace(part), "=", 2)
			if len(kv) == 2 {
				labelMap[kv[0]] = kv[1]
			}
		}

		// Deny all ingress and egress for pods matching the selector
		policy := &networkingv1.NetworkPolicy{
			ObjectMeta: metav1.ObjectMeta{
				Name:      req.PolicyName,
				Namespace: req.Namespace,
			},
			Spec: networkingv1.NetworkPolicySpec{
				PodSelector: metav1.LabelSelector{MatchLabels: labelMap},
				PolicyTypes: []networkingv1.PolicyType{
					networkingv1.PolicyTypeIngress,
					networkingv1.PolicyTypeEgress,
				},
				// Empty Ingress/Egress = deny all
			},
		}

		_, err := clientset.NetworkingV1().NetworkPolicies(req.Namespace).Create(
			context.Background(), policy, metav1.CreateOptions{},
		)
		if err != nil {
			http.Error(w, fmt.Sprintf("failed to create policy: %v", err), http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]string{
			"status":  "blocked",
			"policy":  req.PolicyName,
			"message": fmt.Sprintf("all traffic blocked for %s in %s", req.LabelSelector, req.Namespace),
		})
	} else {
		err := clientset.NetworkingV1().NetworkPolicies(req.Namespace).Delete(
			context.Background(), req.PolicyName, metav1.DeleteOptions{},
		)
		if err != nil {
			http.Error(w, fmt.Sprintf("failed to delete policy: %v", err), http.StatusInternalServerError)
			return
		}

		json.NewEncoder(w).Encode(map[string]string{
			"status":  "unblocked",
			"policy":  req.PolicyName,
			"message": fmt.Sprintf("traffic restored for %s in %s", req.LabelSelector, req.Namespace),
		})
	}
}