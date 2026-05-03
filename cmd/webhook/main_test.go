package main

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http/httptest"
	"os"
	"testing"

	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
)

func TestMain(m *testing.M) {
	slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, nil)))
	os.Exit(m.Run())
}

func TestValidatePod_RejectsPrivileges(t *testing.T) {
	pod := corev1.Pod{
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  "test",
					Image: "test:latest",
					Resources: corev1.ResourceRequirements{
						Limits: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("100m"),
							corev1.ResourceMemory: resource.MustParse("128Mi"),
						},
					},
					SecurityContext: &corev1.SecurityContext{
						Privileged: ptr.To(true),
					},
				},
			},
		},
	}

	body := admissionReviewForPod(t, "tenant-test", pod)
	req := httptest.NewRequest("POST", "/validate-pod", bytes.NewReader(body))
	w := httptest.NewRecorder()
	validatePodHandler(w, req)

	resp := decodeResponse(t, w)
	if resp.Response.Allowed {
		t.Error("Privileged container should be rejected")
	}
}

func TestValidatePod_NoSecurityContext(t *testing.T) {
	pod := corev1.Pod{
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{
				Name:  "test",
				Image: "test:latest",
				Resources: corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("100m"),
						corev1.ResourceMemory: resource.MustParse("128Mi"),
					},
				},
			}},
		},
	}
	body := admissionReviewForPod(t, "tenant-test", pod)
	req := httptest.NewRequest("POST", "/validate-pod", bytes.NewReader(body))
	w := httptest.NewRecorder()
	validatePodHandler(w, req)

	resp := decodeResponse(t, w)
	if resp.Response.Allowed {
		t.Error("Pod without security context should be rejected")
	}
}

func TestValidatePod_RejectRoot(t *testing.T) {
	pod := corev1.Pod{
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{
				Name:  "test",
				Image: "test:latest",
				Resources: corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("100m"),
						corev1.ResourceMemory: resource.MustParse("128Mi"),
					},
				},
				SecurityContext: &corev1.SecurityContext{
					RunAsUser: ptr.To(int64(0)),
				},
			}},
		},
	}
	body := admissionReviewForPod(t, "tenant-test", pod)
	req := httptest.NewRequest("POST", "/validate-pod", bytes.NewReader(body))
	w := httptest.NewRecorder()
	validatePodHandler(w, req)

	resp := decodeResponse(t, w)
	if resp.Response.Allowed {
		t.Error("Pod running as root should be rejected")
	}
}

func TestValidatePod_RejectPrivEscalation(t *testing.T) {
	pod := corev1.Pod{
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{
				Name:  "test",
				Image: "test:latest",
				Resources: corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("100m"),
						corev1.ResourceMemory: resource.MustParse("128Mi"),
					},
				},
				SecurityContext: &corev1.SecurityContext{
					AllowPrivilegeEscalation: ptr.To(true),
					ReadOnlyRootFilesystem:   ptr.To(true),
				},
			}},
		},
	}
	body := admissionReviewForPod(t, "tenant-test", pod)
	req := httptest.NewRequest("POST", "/validate-pod", bytes.NewReader(body))
	w := httptest.NewRecorder()
	validatePodHandler(w, req)

	resp := decodeResponse(t, w)
	if resp.Response.Allowed {
		t.Error("Pod trying to escalate privileges should be rejected")
	}
}

func TestValidatePod_RejectWritrableRootFS(t *testing.T) {
	pod := corev1.Pod{
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{
				Name:  "test",
				Image: "test:latest",
				Resources: corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("100m"),
						corev1.ResourceMemory: resource.MustParse("128Mi"),
					},
				},
				SecurityContext: &corev1.SecurityContext{
					ReadOnlyRootFilesystem: ptr.To(false),
				},
			}},
		},
	}
	body := admissionReviewForPod(t, "tenant-test", pod)
	req := httptest.NewRequest("POST", "/validate-pod", bytes.NewReader(body))
	w := httptest.NewRecorder()
	validatePodHandler(w, req)

	resp := decodeResponse(t, w)
	if resp.Response.Allowed {
		t.Error("Pod trying to set the root fs to writable should be rejected")
	}
}

func TestValidatePod_RejectMissingResourceLimits(t *testing.T) {
	pod := corev1.Pod{
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{
				Name:      "test",
				Image:     "test:latest",
				Resources: corev1.ResourceRequirements{},
			}},
		},
	}
	body := admissionReviewForPod(t, "tenant-test", pod)
	req := httptest.NewRequest("POST", "/validate-pod", bytes.NewReader(body))
	w := httptest.NewRecorder()
	validatePodHandler(w, req)

	resp := decodeResponse(t, w)
	if resp.Response.Allowed {
		t.Error("Pod trying to unset resource limits should be rejected")
	}
}

func TestValidatePod_AdmitsValidPod(t *testing.T) {
	pod := corev1.Pod{
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{
				Name:  "test",
				Image: "test:latest",
				Resources: corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("100m"),
						corev1.ResourceMemory: resource.MustParse("128Mi"),
					},
				},
				SecurityContext: &corev1.SecurityContext{
					Privileged:               ptr.To(false),
					ReadOnlyRootFilesystem:   ptr.To(true),
					AllowPrivilegeEscalation: ptr.To(false),
					RunAsNonRoot:             ptr.To(true),
				},
			}},
		},
	}
	body := admissionReviewForPod(t, "tenant-test", pod)
	req := httptest.NewRequest("POST", "/validate-pod", bytes.NewReader(body))
	w := httptest.NewRecorder()
	validatePodHandler(w, req)

	resp := decodeResponse(t, w)
	if !resp.Response.Allowed {
		t.Error("Pod should be allowed")
	}
}

func TestValidatePod_SkipsNonTenantNamespace(t *testing.T) {
	pod := corev1.Pod{
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  "test",
					Image: "test:latest",
					Resources: corev1.ResourceRequirements{
						Limits: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("100m"),
							corev1.ResourceMemory: resource.MustParse("128Mi"),
						},
					},
					SecurityContext: &corev1.SecurityContext{
						Privileged: ptr.To(true),
					},
				},
			},
		},
	}

	body := admissionReviewForPod(t, "default", pod)
	req := httptest.NewRequest("POST", "/validate-pod", bytes.NewReader(body))
	w := httptest.NewRecorder()
	validatePodHandler(w, req)

	resp := decodeResponse(t, w)
	if !resp.Response.Allowed {
		t.Error("Non-tenant namespace should be admitted regardless of pod config")
	}
}

func TestValidateMutatePod_AddsLabels(t *testing.T) {
	pod := corev1.Pod{
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  "test",
					Image: "test:latest",
				},
			},
		},
	}

	body := admissionReviewForPod(t, "tenant-test-mutate", pod)
	req := httptest.NewRequest("POST", "/mutate-pod", bytes.NewReader(body))
	w := httptest.NewRecorder()
	mutatePodHandler(w, req)

	resp := decodeResponse(t, w)
	if !resp.Response.Allowed {
		t.Error("Pod mutation should be allowed")
	}
	if resp.Response.PatchType == nil || *resp.Response.PatchType != admissionv1.PatchTypeJSONPatch {
		t.Error("expected JSON patch")
	}
	var patches []map[string]any
	if err := json.Unmarshal(resp.Response.Patch, &patches); err != nil {
		t.Fatal(err)
	}
	if len(patches) != 3 {
		t.Fatalf("expected 3 patches, got %d", len(patches))
	}
	if patches[0]["op"] != "add" || patches[0]["path"] != "/metadata/labels" {
		t.Error("value of first patch does not match")
	}
	if patches[1]["op"] != "add" || patches[1]["path"] != "/metadata/labels/tenant.k8s-mtp.io~1name" {
		t.Error("value of second patch does not match")
	}
	if patches[2]["op"] != "add" || patches[2]["path"] != "/metadata/labels/tenant.k8s-mtp.io~1tier" {
		t.Error("value of third patch does not match")
	}
}

func TestValidateMutatePod_SkipsNonTenant(t *testing.T) {
	pod := corev1.Pod{
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  "test",
					Image: "test:latest",
				},
			},
		},
	}

	body := admissionReviewForPod(t, "default", pod)
	req := httptest.NewRequest("POST", "/mutate-pod", bytes.NewReader(body))
	w := httptest.NewRecorder()
	mutatePodHandler(w, req)

	resp := decodeResponse(t, w)
	if !resp.Response.Allowed {
		t.Error("Pod mutation should be allowed")
	}
	if resp.Response.Patch != nil {
		t.Error("expected nil patch for non-tenant namespace")
	}
}

func admissionReviewForPod(t *testing.T, namespace string, pod corev1.Pod) []byte {
	t.Helper()
	podJSON, err := json.Marshal(pod)
	if err != nil {
		t.Fatal(err)
	}
	review := admissionv1.AdmissionReview{
		Request: &admissionv1.AdmissionRequest{
			Namespace: namespace,
			Object:    runtime.RawExtension{Raw: podJSON},
		},
	}
	body, err := json.Marshal(review)
	if err != nil {
		t.Fatal(err)
	}
	return body
}

func decodeResponse(t *testing.T, w *httptest.ResponseRecorder) admissionv1.AdmissionReview {
	t.Helper()
	var review admissionv1.AdmissionReview
	if err := json.NewDecoder(w.Body).Decode(&review); err != nil {
		t.Fatal(err)
	}
	return review
}
