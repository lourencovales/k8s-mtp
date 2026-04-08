package webhook

import (
	"context"
	"fmt"
	"log/slog"

	admissionregistration "k8s.io/api/admissionregistration/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"git.assilvestrar.club/lourenco/k8s-mtp/pkg/webhook"
)

type WebhookManager struct {
	client.Client
	Logger    *slog.Logger
	Namespace string
	Image     string
}

func (w *WebhookManager) EnsureAll(ctx context.Context) error {
	if err := w.ensureTLSSecret(ctx); err != nil {
		return fmt.Errorf("failed to ensure TLS secret: %w", err)
	}

	if err := w.ensureDeployment(ctx); err != nil {
		return fmt.Errorf("failed to ensure deployment: %w", err)
	}

	if err := w.ensureService(ctx); err != nil {
		return fmt.Errorf("failed to ensure service: %w", err)
	}

	if err := w.ensureValidatingWebhookConfig(ctx); err != nil {
		return fmt.Errorf("failed to ensure webhook config: %w", err)
	}

	if err := w.ensureMutatingWebhookConfiguration(ctx); err != nil {
		return fmt.Errorf("failed to ensure mutating webhook config: %w", err)
	}

	return nil
}

func (w *WebhookManager) ensureTLSSecret(ctx context.Context) error {
	secret := &corev1.Secret{}

	err := w.Client.Get(ctx, client.ObjectKey{Name: "webhook-tls", Namespace: w.Namespace}, secret)
	if err == nil {
		w.Logger.Info("TLS secret already exists")
		return nil
	}

	if !apierrors.IsNotFound(err) {
		return fmt.Errorf("failed to get secret: %w", err)
	}

	w.Logger.Info("Generating TLS cert")

	caCert, caKey, err := webhook.GenerateCA()
	if err != nil {
		return fmt.Errorf("error generating CA certs: %w", err)
	}

	serverCert, serverKey, err := webhook.GenerateServerCert(caCert, caKey)
	if err != nil {
		return fmt.Errorf("error generating server certs: %w", err)
	}

	newSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "webhook-tls",
			Namespace: w.Namespace,
		},
		Data: map[string][]byte{
			"ca.crt":  caCert,
			"tls.crt": serverCert,
			"tls.key": serverKey,
		},
		Type: corev1.SecretTypeTLS,
	}

	if err := w.Client.Create(ctx, newSecret); err != nil {
		return fmt.Errorf("failed to create secret: %w", err)
	}

	w.Logger.Info("TLS secret created successfully")
	return nil
}

func (w *WebhookManager) ensureDeployment(ctx context.Context) error {
	deployment := &appsv1.Deployment{}

	err := w.Client.Get(ctx, client.ObjectKey{Name: "webhook", Namespace: w.Namespace}, deployment)
	if err == nil {
		w.Logger.Info("Deployment already exists")
		return nil
	}

	if !apierrors.IsNotFound(err) {
		return fmt.Errorf("failed to get deployment: %w", err)
	}

	w.Logger.Info("Creating deployment")

	newDeployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "webhook",
			Namespace: w.Namespace,
			Labels: map[string]string{
				"app": "webhook",
			},
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": "webhook",
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": "webhook",
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:            "webhook",
							Image:           w.Image,
							ImagePullPolicy: corev1.PullIfNotPresent,
							Ports: []corev1.ContainerPort{
								{
									ContainerPort: 8443,
									Protocol:      corev1.ProtocolTCP,
								},
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "webhook-tls",
									MountPath: "/etc/webhook/tls",
									ReadOnly:  true,
								},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "webhook-tls",
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName: "webhook-tls",
								},
							},
						},
					},
				},
			},
		},
	}

	if err := w.Client.Create(ctx, newDeployment); err != nil {
		return fmt.Errorf("failed to create deployment: %w", err)
	}

	w.Logger.Info("Deployment created")

	return nil
}

func (w *WebhookManager) ensureService(ctx context.Context) error {
	service := &corev1.Service{}
	err := w.Client.Get(ctx, client.ObjectKey{Name: "webhook-svc", Namespace: w.Namespace}, service)
	if err == nil {
		w.Logger.Info("Service already exists")
		return nil
	}

	if !apierrors.IsNotFound(err) {
		return fmt.Errorf("failed to get service: %w", err)
	}

	w.Logger.Info("Creating service")

	newService := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "webhook-svc",
			Namespace: w.Namespace,
			Labels: map[string]string{
				"app": "webhook",
			},
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				{
					Port:       443,
					TargetPort: intstr.FromInt(8443),
				},
			},
			Selector: map[string]string{
				"app": "webhook",
			},
		},
	}

	if err := w.Client.Create(ctx, newService); err != nil {
		return fmt.Errorf("failed to create service: %w", err)
	}

	w.Logger.Info("Service created")

	return nil
}

func (w *WebhookManager) ensureValidatingWebhookConfig(ctx context.Context) error {
	secret := &corev1.Secret{}

	err := w.Client.Get(ctx, client.ObjectKey{Name: "webhook-tls", Namespace: w.Namespace}, secret)
	if err != nil {
		return fmt.Errorf("error getting secret: %w", err)
	}

	caCert := secret.Data["ca.crt"]

	config := &admissionregistration.ValidatingWebhookConfiguration{}
	err = w.Client.Get(ctx, client.ObjectKey{Name: "k8s-mtp-webhook"}, config)
	if err == nil {
		w.Logger.Info("Webhook validation done")
		return nil
	}

	if !apierrors.IsNotFound(err) {
		return fmt.Errorf("failed to get webhook config validation: %w", err)
	}

	w.Logger.Info("Creating webhook config validation")

	newConfig := &admissionregistration.ValidatingWebhookConfiguration{
		ObjectMeta: metav1.ObjectMeta{
			Name: "k8s-mtp-webhook",
		},
		Webhooks: []admissionregistration.ValidatingWebhook{
			{
				Name: "k8s-mtp-webhook.k8s-mtp.io",
				ClientConfig: admissionregistration.WebhookClientConfig{
					Service: &admissionregistration.ServiceReference{
						Name:      "webhook-svc",
						Namespace: w.Namespace,
						Path:      ptr.To("/validate-pod"),
					},
					CABundle: caCert,
				},
				Rules: []admissionregistration.RuleWithOperations{
					{
						Operations: []admissionregistration.OperationType{"CREATE", "UPDATE"},
						Rule: admissionregistration.Rule{
							APIGroups:   []string{""},
							APIVersions: []string{"v1"},
							Resources:   []string{"pods"},
						},
					},
				},
				FailurePolicy:           ptr.To(admissionregistration.Fail),
				SideEffects:             ptr.To(admissionregistration.SideEffectClassNone),
				AdmissionReviewVersions: []string{"v1"},
				NamespaceSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"tenant.k8s-mtp.io/managed": "true",
					},
				},
			},
		},
	}

	if err := w.Client.Create(ctx, newConfig); err != nil {
		return fmt.Errorf("failed to create webhook config: %w", err)
	}
	w.Logger.Info("Webhook config created")

	return nil
}

func (w *WebhookManager) ensureMutatingWebhookConfiguration(ctx context.Context) error {
	secret := &corev1.Secret{}

	err := w.Client.Get(ctx, client.ObjectKey{Name: "webhook-tls", Namespace: w.Namespace}, secret)
	if err != nil {
		return fmt.Errorf("error getting secret: %w", err)
	}

	caCert := secret.Data["ca.crt"]

	config := &admissionregistration.MutatingWebhookConfiguration{}
	err = w.Client.Get(ctx, client.ObjectKey{Name: "k8s-mtp-mutating-webhook"}, config)
	if err == nil {
		w.Logger.Info("Webhook validation done")
		return nil
	}

	if !apierrors.IsNotFound(err) {
		return fmt.Errorf("failed to get mutating webhook config validation: %w", err)
	}

	w.Logger.Info("Creating mutating webhook config validation")

	newConfig := &admissionregistration.MutatingWebhookConfiguration{
		ObjectMeta: metav1.ObjectMeta{
			Name: "k8s-mtp-mutating-webhook",
		},
		Webhooks: []admissionregistration.MutatingWebhook{
			{
				Name: "k8s-mtp-mutating-webhook.k8s-mtp.io",
				ClientConfig: admissionregistration.WebhookClientConfig{
					Service: &admissionregistration.ServiceReference{
						Name:      "webhook-svc",
						Namespace: w.Namespace,
						Path:      ptr.To("/mutate-pod"),
					},
					CABundle: caCert,
				},
				Rules: []admissionregistration.RuleWithOperations{
					{
						Operations: []admissionregistration.OperationType{"CREATE", "UPDATE"},
						Rule: admissionregistration.Rule{
							APIGroups:   []string{""},
							APIVersions: []string{"v1"},
							Resources:   []string{"pods"},
						},
					},
				},
				FailurePolicy:           ptr.To(admissionregistration.Fail),
				SideEffects:             ptr.To(admissionregistration.SideEffectClassNone),
				AdmissionReviewVersions: []string{"v1"},
				ReinvocationPolicy:      ptr.To(admissionregistration.NeverReinvocationPolicy),
				NamespaceSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"tenant.k8s-mtp.io/managed": "true",
					},
				},
			},
		},
	}

	if err := w.Client.Create(ctx, newConfig); err != nil {
		return fmt.Errorf("failed to create mutating webhook config: %w", err)
	}
	w.Logger.Info("Mutating webhook config created")

	return nil
}
