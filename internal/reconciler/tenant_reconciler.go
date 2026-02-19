// Package reconciler implements the Tenant CRD reconciler for the k8s-mtp platform.
// It handles namespace creation, ResourceQuota enforcement, and tenant lifecycle management.
package reconciler

import (
	"context"
	"fmt"
	"log/slog"

	corev1 "k8s.io/api/core/v1"
	errors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "git.assilvestrar.club/lourenco/k8s-mtp/pkg/api/v1"
)

type TenantReconciler struct {
	client.Client
	Logger *slog.Logger // TODO: add logger through the package
}

func (r *TenantReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).For(&v1.Tenant{}).Owns(&corev1.Namespace{}).Complete(r)
}

func (r *TenantReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	tenant := &v1.Tenant{}
	if err := r.Get(ctx, req.NamespacedName, tenant); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	desiredNamespace := r.buildNamespace(tenant)
	desiredQuota := r.buildResourceQuota(tenant)

	if err := r.createOrUpdate(ctx, desiredNamespace); err != nil {
		return ctrl.Result{}, err
	}

	if err := r.createOrUpdate(ctx, desiredQuota); err != nil {
		return ctrl.Result{}, err
	}

	tenant.Status.Phase = v1.TenantPhaseActive
	tenant.Status.Namespace = desiredNamespace.Name

	if err := r.Client.Status().Update(ctx, tenant); err != nil {
		return ctrl.Result{}, err
	}

	// TODO: requeue?

	return ctrl.Result{}, nil
}

func (r *TenantReconciler) buildNamespace(t *v1.Tenant) *corev1.Namespace {
	return &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: fmt.Sprintf("tenant-%s-%s", t.Name, t.Spec.Name),
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: v1.SchemeGroupVersion.String(),
					Kind:       "Tenant",
					Name:       t.Name,
					UID:        t.UID,
				},
			},
			Labels: map[string]string{},
		},
	}
}

func (r *TenantReconciler) buildResourceQuota(t *v1.Tenant) *corev1.ResourceQuota {
	// for now we're just building for the lowest tier
	return &corev1.ResourceQuota{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "tenant-quota",
			Namespace: fmt.Sprintf("tenant-%s-%s", t.Name, t.Spec.Name),
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: v1.SchemeGroupVersion.String(),
					Kind:       "Tenant",
					Name:       t.Name,
					UID:        t.UID,
				},
			},
		},
		Spec: corev1.ResourceQuotaSpec{
			Hard: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("500m"),
				corev1.ResourceMemory: resource.MustParse("1Gi"),
				corev1.ResourcePods:   resource.MustParse("20"),
			},
		},
	}
}

func (r *TenantReconciler) createOrUpdate(ctx context.Context, obj client.Object) error {
	if err := r.Create(ctx, obj); err != nil {
		if !errors.IsAlreadyExists(err) {
			return fmt.Errorf("failed to create: %w", err)
		}

		key := client.ObjectKeyFromObject(obj)
		current := obj.DeepCopyObject().(client.Object)

		if err := r.Get(ctx, key, current); err != nil {
			return fmt.Errorf("failed to get existing: %w", err)
		}

		if !needsUpdate(current, obj) {
			return nil
		}

		obj.SetResourceVersion(current.GetResourceVersion())

		if err := r.Update(ctx, obj); err != nil {
			return fmt.Errorf("failed to update: %w", err)
		}
	}
	return nil
}

func needsUpdate(current, desired client.Object) bool {
	switch desired := desired.(type) {
	case *corev1.Namespace:
		return nsNeedsUpdate(current.(*corev1.Namespace), desired)
	case *corev1.ResourceQuota:
		return rqNeedsUpdate(current.(*corev1.ResourceQuota), desired)
	default:
		return true
	}
}

func nsNeedsUpdate(current, desired *corev1.Namespace) bool {
	if len(current.Annotations) != len(desired.Annotations) {
		return true
	}
	for k, v := range desired.Annotations {
		if current.Annotations[k] != v {
			return true
		}
	}

	if len(current.OwnerReferences) != len(desired.OwnerReferences) {
		return true
	}
	for _, desiredRef := range desired.OwnerReferences {
		found := false
		for _, currentRef := range current.OwnerReferences {
			if currentRef.UID == desiredRef.UID {
				found = true
				if currentRef.APIVersion != desiredRef.APIVersion ||
					currentRef.Kind != desiredRef.Kind ||
					currentRef.Name != desiredRef.Name {
					return true
				}
				break
			}
		}
		if !found {
			return true
		}
	}

	if len(current.Labels) != len(desired.Labels) {
		return true
	}
	for k, v := range desired.Labels {
		if current.Labels[k] != v {
			return true
		}
	}
	return false
}

func rqNeedsUpdate(current, desired *corev1.ResourceQuota) bool {
	if len(current.Spec.Hard) != len(desired.Spec.Hard) {
		return true
	}
	for k, v := range desired.Spec.Hard {
		if !current.Spec.Hard[k].Equal(v) {
			return true
		}
	}

	if len(current.Annotations) != len(desired.Annotations) {
		return true
	}
	for k, v := range desired.Annotations {
		if current.Annotations[k] != v {
			return true
		}
	}

	if len(current.Labels) != len(desired.Labels) {
		return true
	}
	for k, v := range desired.Labels {
		if current.Labels[k] != v {
			return true
		}
	}

	if len(current.OwnerReferences) != len(desired.OwnerReferences) {
		return true
	}
	for _, desiredRef := range desired.OwnerReferences {
		found := false
		for _, currentRef := range current.OwnerReferences {
			if currentRef.UID == desiredRef.UID {
				found = true
				if currentRef.APIVersion != desiredRef.APIVersion ||
					currentRef.Kind != desiredRef.Kind ||
					currentRef.Name != desiredRef.Name {
					return true
				}
				break
			}
		}
		if !found {
			return true
		}
	}
	return false
}
