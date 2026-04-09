package main

import (
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"

	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
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
	mux.HandleFunc("/mutate-pod", mutatePodHandler)

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

	if !shouldValidateNamespace(namespace) {
		response := admissionv1.AdmissionReview{
			Response: &admissionv1.AdmissionResponse{
				UID:     uid,
				Allowed: true,
				Result:  &metav1.Status{Status: "Success"},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		err = json.NewEncoder(w).Encode(response)
		if err != nil {
			slog.Error("Failed to encode answer to K8s API", "error", err)
			return
		}
		return
	}

	var pod corev1.Pod
	err = json.Unmarshal(podJSON, &pod)
	if err != nil {
		slog.Error("Failed to unmarshal body of the raw K8s API admission review request", "error", err)
		http.Error(w, "Failed to unmarshal json", http.StatusBadRequest)
		return
	}

	allowed, reason := validatePodResource(&pod)

	response := admissionv1.AdmissionReview{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "admission.k8s.io/v1",
			Kind:       "AdmissionReview",
		},
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

func mutatePodHandler(w http.ResponseWriter, r *http.Request) {
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

	if !shouldValidateNamespace(namespace) {
		response := admissionv1.AdmissionReview{
			Response: &admissionv1.AdmissionResponse{
				UID:     uid,
				Allowed: true,
				Result:  &metav1.Status{Status: "Success"},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		err = json.NewEncoder(w).Encode(response)
		if err != nil {
			slog.Error("Failed to encode answer to K8s API", "error", err)
			return
		}
		return
	}

	patches := []map[string]any{
		{
			"op":    "add",
			"path":  "/metadata/labels/tenant.k8s-mtp.io~1name",
			"value": strings.Split(namespace, "-")[1],
		},
		{
			"op":    "add",
			"path":  "/metadata/labels/tenant.k8s-mtp.io~1tier",
			"value": strings.Split(namespace, "-")[2],
		},
	}

	jsonPatch, err := json.Marshal(patches)
	if err != nil {
		slog.Error("Failed to marshal patch to K8s API", "error", err)
		return
	}

	encPatch := make([]byte, base64.StdEncoding.EncodedLen(len(jsonPatch)))
	base64.StdEncoding.Encode(encPatch, jsonPatch)

	response := &admissionv1.AdmissionResponse{
		UID:       uid,
		Allowed:   true,
		Patch:     encPatch,
		PatchType: ptr.To(admissionv1.PatchTypeJSONPatch),
	}

	rev := admissionv1.AdmissionReview{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "admission.k8s.io/v1",
			Kind:       "AdmissionReview",
		},
		Response: response,
	}

	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(rev)
	if err != nil {
		slog.Error("Failed to encode answer to K8s API", "error", err)
		return
	}
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "healthy"})
}

func validatePodResource(pod *corev1.Pod) (bool, metav1.Status) {
	for _, container := range pod.Spec.Containers {
		if allowed, status := validateContainer(container); !allowed {
			return allowed, status
		}
	}

	for _, container := range pod.Spec.InitContainers {
		if allowed, status := validateContainer(container); !allowed {
			return allowed, status
		}
	}

	for _, ephemeral := range pod.Spec.EphemeralContainers {
		if allowed, status := validateEphemeralContainer(ephemeral); !allowed {
			return allowed, status
		}
	}

	return true, metav1.Status{Status: "Success"}
}

func validateContainer(container corev1.Container) (bool, metav1.Status) {
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

	if container.SecurityContext == nil {
		return false, metav1.Status{
			Status:  "Failure",
			Message: fmt.Sprintf("Container %s must have SecurityContext defined", container.Name),
			Reason:  "MissingSecurityContext",
			Code:    400,
		}
	}

	if (container.SecurityContext.Privileged != nil && *container.SecurityContext.Privileged) ||
		(container.SecurityContext.RunAsNonRoot != nil && !*container.SecurityContext.RunAsNonRoot) ||
		(container.SecurityContext.RunAsUser != nil && *container.SecurityContext.RunAsUser == 0) {
		return false, metav1.Status{
			Status:  "Failure",
			Message: fmt.Sprintf("Container %s failed privilege check", container.Name),
			Reason:  "Conflict",
			Code:    400,
		}
	}

	if (container.SecurityContext.ReadOnlyRootFilesystem != nil &&
		!*container.SecurityContext.ReadOnlyRootFilesystem) ||
		container.SecurityContext.ReadOnlyRootFilesystem == nil {
		return false, metav1.Status{
			Status:  "Failure",
			Message: fmt.Sprintf("Container %s must have read only root filesystem", container.Name),
			Reason:  "Conflict",
			Code:    400,
		}
	}

	if (container.SecurityContext.AllowPrivilegeEscalation != nil &&
		*container.SecurityContext.AllowPrivilegeEscalation) ||
		container.SecurityContext.AllowPrivilegeEscalation == nil {
		return false, metav1.Status{
			Status:  "Failure",
			Message: fmt.Sprintf("Container %s must not be allowed to escalete privileges", container.Name),
			Reason:  "Conflict",
			Code:    400,
		}
	}
	return true, metav1.Status{}
}

// TODO: DRY
func validateEphemeralContainer(container corev1.EphemeralContainer) (bool, metav1.Status) {
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

	if container.SecurityContext == nil {
		return false, metav1.Status{
			Status:  "Failure",
			Message: fmt.Sprintf("Container %s must have SecurityContext defined", container.Name),
			Reason:  "MissingSecurityContext",
			Code:    400,
		}
	}

	if (container.SecurityContext.Privileged != nil && *container.SecurityContext.Privileged) ||
		(container.SecurityContext.RunAsNonRoot != nil && !*container.SecurityContext.RunAsNonRoot) ||
		(container.SecurityContext.RunAsUser != nil && *container.SecurityContext.RunAsUser == 0) {
		return false, metav1.Status{
			Status:  "Failure",
			Message: fmt.Sprintf("Container %s failed privilege check", container.Name),
			Reason:  "Conflict",
			Code:    400,
		}
	}

	if (container.SecurityContext.ReadOnlyRootFilesystem != nil &&
		!*container.SecurityContext.ReadOnlyRootFilesystem) ||
		container.SecurityContext.ReadOnlyRootFilesystem == nil {
		return false, metav1.Status{
			Status:  "Failure",
			Message: fmt.Sprintf("Container %s must have read only root filesystem", container.Name),
			Reason:  "Conflict",
			Code:    400,
		}
	}

	if (container.SecurityContext.AllowPrivilegeEscalation != nil &&
		*container.SecurityContext.AllowPrivilegeEscalation) ||
		container.SecurityContext.AllowPrivilegeEscalation == nil {
		return false, metav1.Status{
			Status:  "Failure",
			Message: fmt.Sprintf("Container %s must not be allowed to escalete privileges", container.Name),
			Reason:  "Conflict",
			Code:    400,
		}
	}
	return true, metav1.Status{}
}

func shouldValidateNamespace(ns string) bool {
	systemNamespace := map[string]bool{
		"kube-system":     true,
		"kube-public":     true,
		"kube-node-lease": true,
		"default":         true,
		"k8s-mtp":         true,
	}
	if systemNamespace[ns] {
		return false
	}

	return strings.HasPrefix(ns, "tenant-")
}
