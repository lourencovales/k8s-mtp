package webhook

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"

	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	certFile = "/etc/webhook/tls/tls.crt"
	keyFile  = "/etc/webhook/tls/tls.key"
	port     = ":8443"
)

func main() {
	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		slog.Error("Failed to load TLS cert", "error", err)
		os.Exit(1)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/validate-pod", validatePodHandler)
	mux.HandleFunc("/healthz", healthHandler)

	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{cert},
	}

	server := &http.Server{
		Addr:      port,
		TLSConfig: tlsConfig,
		Handler:   mux,
	}

	slog.Info("Starting webhook server", "port", port)
	if err := server.ListenAndServeTLS("", ""); err != nil {
		slog.Error("Server failed", "error", err)
		os.Exit(1)
	}
}

func validatePodHandler(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		slog.Error("Faied to read body of the K8s API admission review request", "error", err)
		http.Error(w, "Failed to read body", http.StatusBadRequest)
		return
	}

	var review admissionv1.AdmissionReview
	err = json.Unmarshal(body, &review)
	if err != nil {
		slog.Error("Failed to unmarshal json of the K8s API admission review request", "error", err)
		http.Error(w, "Failed to unmarshal json", http.StatusBadRequest)
		return
	}

	uid := review.Request.UID
	namespace := review.Request.Namespace
	podJSON := review.Request.Object.Raw

	var pod corev1.Pod
	err = json.Unmarshal(podJSON, &pod)
	if err != nil {
		slog.Error("Failed to unmarshal body of the raw K8s API admission review request", "error", err)
		http.Error(w, "Failed to unmarshal json", http.StatusBadRequest)
		return
	}

	allowed, reason := validatePodResource(&pod)

	response := admissionv1.AdmissionReview{
		Response: &admissionv1.AdmissionResponse{
			UID:     uid,
			Allowed: allowed,
			Result: &metav1.Status{
				Message: reason.Message,
			},
		},
	}

	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(response)
	if err != nil {
		slog.Error("Failed to encode answer to K8s API", "error", err)
		return
	}

	slog.Info("Pod validation",
		"namespace", namespace,
		"name", pod.Name,
		"allowed", allowed,
		"reason", reason,
	)
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "healthy"})
}

func validatePodResource(pod *corev1.Pod) (bool, metav1.Status) {
	for _, container := range pod.Spec.Containers {
		if container.Resources.Limits == nil {
			return false, metav1.Status{
				Status:  "Failure",
				Message: fmt.Sprintf("Container %s must have resource limits", container.Name),
				Reason:  "MissingResourceLimits",
				Code:    400,
			}
		}

		if container.Resources.Limits.Cpu().IsZero() {
			return false, metav1.Status{
				Status:  "Failure",
				Message: fmt.Sprintf("Container %s must have CPU limits", container.Name),
				Reason:  "MissingCPULimit",
				Code:    400,
			}
		}

		if container.Resources.Limits.Memory().IsZero() {
			return false, metav1.Status{
				Status:  "Failure",
				Message: fmt.Sprintf("Container %s must have Memory limits", container.Name),
				Reason:  "MissingMemoryLimit",
				Code:    400,
			}
		}

		if container.Resources.Limits.Storage().IsZero() {
			return false, metav1.Status{
				Status:  "Failure",
				Message: fmt.Sprintf("Container %s must have Storage limits", container.Name),
				Reason:  "MissingStorageLimit",
				Code:    400,
			}
		}
	}
	return true, metav1.Status{Status: "Success"}
}
