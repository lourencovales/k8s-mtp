// Package reconciler implements the Tenant CRD reconciler for the k8s-mtp platform.
// It handles namespace creation, ResourceQuota enforcement, and tenant lifecycle management.
package reconciler

import (
	"context"
	"fmt"
	"log/slog"
	"slices"
	"strings"

	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	errors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"git.assilvestrar.club/lourenco/k8s-mtp/internal/config"
	"git.assilvestrar.club/lourenco/k8s-mtp/internal/store"
	v1 "git.assilvestrar.club/lourenco/k8s-mtp/pkg/api/v1"
)

type TenantReconciler struct {
	client.Client
	Logger *slog.Logger // TODO: add logger through the package
	Store  *store.Database
	Config *config.Config
}

const tenantFinalizer = "tenant.multitenant.k8s-mtp.io/finalizer"

func (r *TenantReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).For(&v1.Tenant{}).Owns(&corev1.Namespace{}).Owns(&rbacv1.Role{}).Owns(&rbacv1.RoleBinding{}).Complete(r)
}

func (r *TenantReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	tenant := &v1.Tenant{}
	if err := r.Get(ctx, req.NamespacedName, tenant); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if !tenant.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, tenant)
	}

	if !slices.Contains(tenant.Finalizers, tenantFinalizer) {
		return r.addFinalizer(ctx, tenant)
	}

	return r.reconcileCreateOrUpdate(ctx, tenant)
}

func (r *TenantReconciler) addFinalizer(ctx context.Context, tenant *v1.Tenant) (ctrl.Result, error) {
	tenant.Finalizers = append(tenant.Finalizers, tenantFinalizer)
	if err := r.Update(ctx, tenant); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{Requeue: true}, nil
}

func (r *TenantReconciler) reconcileDelete(ctx context.Context, tenant *v1.Tenant) (ctrl.Result, error) {
	if err := r.Store.DeleteTenant(ctx, string(tenant.UID)); err != nil {
		return ctrl.Result{}, err
	}

	tenant.Finalizers = removeString(tenant.Finalizers, tenantFinalizer)
	if err := r.Update(ctx, tenant); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *TenantReconciler) reconcileCreateOrUpdate(ctx context.Context, tenant *v1.Tenant) (ctrl.Result, error) {
	existing, err := r.Store.GetTenantByID(ctx, string(tenant.UID))
	if err != nil {
		return ctrl.Result{}, err
	}

	if existing == nil {
		if err := r.Store.CreateTenant(ctx, tenant); err != nil {
			return ctrl.Result{}, err
		}
	} else {
		if err := r.Store.UpdateTenant(ctx, tenant); err != nil {
			return ctrl.Result{}, err
		}
	}

	namespace := r.buildNamespace(tenant)
	quota := r.buildResourceQuota(tenant)

	if err := r.createOrUpdate(ctx, namespace); err != nil {
		return ctrl.Result{}, err
	}

	if err := r.createOrUpdate(ctx, quota); err != nil {
		return ctrl.Result{}, err
	}

	if err := r.createOrUpdate(ctx, buildAdminRole(tenant)); err != nil {
		return ctrl.Result{}, err
	}

	if err := r.createOrUpdate(ctx, buildOperatorRole(tenant)); err != nil {
		return ctrl.Result{}, err
	}

	if err := r.createOrUpdate(ctx, buildAdminRoleBinding(tenant)); err != nil {
		return ctrl.Result{}, err
	}

	if err := r.createOrUpdate(ctx, buildOperatorRoleBinding(tenant)); err != nil {
		return ctrl.Result{}, err
	}

	tenant.Status.Phase = v1.TenantPhaseActive
	tenant.Status.Namespace = namespace.Name

	if err := r.Client.Status().Update(ctx, tenant); err != nil {
		return ctrl.Result{}, err
	}

	networkPolicy := r.buildNetworkPolicy(tenant)
	if err := r.createOrUpdate(ctx, networkPolicy); err != nil {
		return ctrl.Result{}, err
	}

	limitRange := r.buildLimitRange(tenant)
	if err := r.createOrUpdate(ctx, limitRange); err != nil {
		return ctrl.Result{}, err
	}

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
	switch t.Spec.Tier {
	case v1.TierFree:
		return buildFreeTier(t)
	case v1.TierPro:
		return buildProTier(t)
	case v1.TierEnterprise:
		return buildEnterpriseTier(t)
	default:
		return nil
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
	case *rbacv1.Role:
		return roleNeedsUpdate(current.(*rbacv1.Role), desired)
	case *rbacv1.RoleBinding:
		return rbNeedsUpdate(current.(*rbacv1.RoleBinding), desired)
	case *networkingv1.NetworkPolicy:
		return npNeedsUpdate(current.(*networkingv1.NetworkPolicy), desired)
	case *corev1.LimitRange:
		return lrNeedsUpdate(current.(*corev1.LimitRange), desired)
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

func roleNeedsUpdate(current, desired *rbacv1.Role) bool {
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

	if len(current.Rules) != len(desired.Rules) {
		return true
	}

	currentRules := make(map[string]bool)
	for _, rule := range current.Rules {
		key := normalizeRule(rule)
		currentRules[key] = true
	}

	desiredRules := make(map[string]bool)
	for _, rule := range desired.Rules {
		key := normalizeRule(rule)
		desiredRules[key] = true
	}

	for key := range desiredRules {
		if !currentRules[key] {
			return true
		}
	}

	for key := range currentRules {
		if !desiredRules[key] {
			return true
		}
	}

	return false
}

func rbNeedsUpdate(current, desired *rbacv1.RoleBinding) bool {
	if len(current.Annotations) != len(desired.Annotations) {
		return true
	}
	for k, v := range desired.Annotations {
		if current.Annotations[k] != v {
			return true
		}
	}

	if len(current.Subjects) != len(desired.Subjects) {
		return true
	}
	currentSubjects := make(map[string]bool)
	for _, sub := range current.Subjects {
		key := fmt.Sprintf("%s|%s|%s", sub.Kind, sub.APIGroup, sub.Name)
		currentSubjects[key] = true
	}
	for _, sub := range desired.Subjects {
		key := fmt.Sprintf("%s|%s|%s", sub.Kind, sub.APIGroup, sub.Name)
		if !currentSubjects[key] {
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

	if current.RoleRef.APIGroup != desired.RoleRef.APIGroup ||
		current.RoleRef.Kind != desired.RoleRef.Kind ||
		current.RoleRef.Name != desired.RoleRef.Name {
		return true
	}

	return false
}

func npNeedsUpdate(current, desired *networkingv1.NetworkPolicy) bool {
	if len(current.Spec.PolicyTypes) != len(desired.Spec.PolicyTypes) {
		return true
	}

	if len(current.Spec.Ingress) != len(desired.Spec.Ingress) {
		return true
	}

	if len(current.Spec.Egress) != len(desired.Spec.Egress) {
		return true
	}

	return false
}

func lrNeedsUpdate(current, desired *corev1.LimitRange) bool {
	if len(current.Spec.Limits) != len(desired.Spec.Limits) {
		return true
	}
	for i := range desired.Spec.Limits {
		if len(current.Spec.Limits[i].Default) != len(desired.Spec.Limits[i].Default) {
			return true
		}
		for k, v := range desired.Spec.Limits[i].Default {
			if !current.Spec.Limits[i].Default[k].Equal(v) {
				return true
			}
		}
	}
	return false
}

func buildAdminRole(t *v1.Tenant) *rbacv1.Role {
	return &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("tenant-%s-admin", t.Name),
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
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{""},
				Resources: []string{
					"pods", "pods/logs", "pods/status", "pods/exec",
					"services", "services/proxy", "endpoints",
					"configmaps", "secrets", "serviceaccounts",
					"events", "limitranges", "podtemplates",
				},
				Verbs: []string{
					"get", "list", "watch", "create", "update",
					"patch", "delete", "deletecollection",
				},
			},
			{
				APIGroups: []string{"apps"},
				Resources: []string{
					"deployments", "deployments/scale", "deployments/status",
					"replicasets", "replicasets/scale", "replicasets/status",
					"daemonsets", "daemonsets/status",
					"statefulsets", "statefulsets/scale", "statefulsets/status",
				},
				Verbs: []string{
					"get", "list", "watch", "create", "update", "patch",
					"delete", "deletecollection",
				},
			},
			{
				APIGroups: []string{"batch"},
				Resources: []string{
					"jobs", "jobs/status", "cronjobs", "cronjobs/status",
				},
				Verbs: []string{
					"get", "list", "watch", "create", "update", "patch",
					"delete", "deletecollection",
				},
			},
			{
				APIGroups: []string{"autoscaling"},
				Resources: []string{"horizontalpodautoscalers"},
				Verbs: []string{
					"get", "list", "watch", "create", "update",
					"patch", "delete",
				},
			},
			{
				APIGroups: []string{},
				Resources: []string{"bindings"},
				Verbs:     []string{"create"},
			},
		},
	}
}

func buildOperatorRole(t *v1.Tenant) *rbacv1.Role {
	return &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("tenant-%s-operator", t.Name),
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
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{""},
				Resources: []string{
					"pods", "pods/log", "pods/status", "pods/exec",
					"services", "endpoints",
					"configmaps", "secrets",
					"events",
				},
				Verbs: []string{
					"get", "list", "watch",
					"create", "update", "patch",
					"delete", // Can delete individual resources
				},
			},
			{
				APIGroups: []string{"apps"},
				Resources: []string{
					"deployments", "deployments/scale", "deployments/status",
					"replicasets", "replicasets/scale",
				},
				Verbs: []string{
					"get", "list", "watch",
					"create", "update", "patch",
					"delete",
				},
			},
			{
				APIGroups: []string{"batch"},
				Resources: []string{"jobs", "jobs/status"},
				Verbs: []string{
					"get", "list", "watch",
					"create", "update", "patch",
					"delete",
				},
			},
		},
	}
}

func buildAdminRoleBinding(t *v1.Tenant) *rbacv1.RoleBinding {
	return &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("tenant-%s-admin-binding", t.Name),
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
		Subjects: []rbacv1.Subject{
			{
				Kind:     "Group",
				APIGroup: "rbac.authorization.k8s.io",
				Name:     fmt.Sprintf("tenant-%s-admins", t.Name),
			},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "Role",
			Name:     fmt.Sprintf("tenant-%s-admin", t.Name),
		},
	}
}

func buildOperatorRoleBinding(t *v1.Tenant) *rbacv1.RoleBinding {
	return &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("tenant-%s-operator-binding", t.Name),
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
		Subjects: []rbacv1.Subject{
			{
				Kind:     "Group",
				APIGroup: "rbac.authorization.k8s.io",
				Name:     fmt.Sprintf("tenant-%s-operators", t.Name),
			},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "Role",
			Name:     fmt.Sprintf("tenant-%s-operator", t.Name),
		},
	}
}

func normalizeRule(rule rbacv1.PolicyRule) string {
	apiGroups := make([]string, len(rule.APIGroups))
	copy(apiGroups, rule.APIGroups)
	slices.Sort(apiGroups)

	resources := make([]string, len(rule.Resources))
	copy(resources, rule.Resources)
	slices.Sort(resources)

	verbs := make([]string, len(rule.Verbs))
	copy(verbs, rule.Verbs)
	slices.Sort(verbs)

	names := make([]string, len(rule.ResourceNames))
	copy(names, rule.ResourceNames)
	slices.Sort(names)

	return strings.Join(apiGroups, ",") + "|" +
		strings.Join(resources, ",") + "|" +
		strings.Join(verbs, ",")
}

func buildFreeTier(t *v1.Tenant) *corev1.ResourceQuota {
	return &corev1.ResourceQuota{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "tenant-quota-free",
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
				corev1.ResourceCPU:     resource.MustParse("500m"),
				corev1.ResourceMemory:  resource.MustParse("1Gi"),
				corev1.ResourcePods:    resource.MustParse("20"),
				corev1.ResourceStorage: resource.MustParse("10Gi"),
			},
		},
	}
}

func buildProTier(t *v1.Tenant) *corev1.ResourceQuota {
	return &corev1.ResourceQuota{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "tenant-quota-pro",
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
				corev1.ResourceCPU:     resource.MustParse("4"),
				corev1.ResourceMemory:  resource.MustParse("8Gi"),
				corev1.ResourcePods:    resource.MustParse("100"),
				corev1.ResourceStorage: resource.MustParse("100Gi"),
			},
		},
	}
}

func buildEnterpriseTier(t *v1.Tenant) *corev1.ResourceQuota {
	return &corev1.ResourceQuota{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "tenant-quota-enterprise",
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
				corev1.ResourceCPU:     resource.MustParse("8"),
				corev1.ResourceMemory:  resource.MustParse("16Gi"),
				corev1.ResourcePods:    resource.MustParse("200"),
				corev1.ResourceStorage: resource.MustParse("200Gi"),
			},
		},
	}
}

func removeString(slice []string, s string) []string {
	result := []string{}
	for _, item := range slice {
		if item != s {
			result = append(result, item)
		}
	}
	return result
}

func (r *TenantReconciler) buildNetworkPolicy(t *v1.Tenant) *networkingv1.NetworkPolicy {
	ns := fmt.Sprintf("tenant-%s-%s", t.Name, t.Spec.Name)

	ingressRules := []networkingv1.NetworkPolicyIngressRule{
		{
			From: []networkingv1.NetworkPolicyPeer{
				{
					PodSelector: &metav1.LabelSelector{},
				},
			},
		},
		{
			From: []networkingv1.NetworkPolicyPeer{
				{
					NamespaceSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"name": "k8s-mtp",
						},
					},
					PodSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							r.Config.PlatformAccessLabel: "true",
						},
					},
				},
			},
		},
	}

	egressRules := []networkingv1.NetworkPolicyEgressRule{
		{
			To: []networkingv1.NetworkPolicyPeer{
				{
					PodSelector: &metav1.LabelSelector{},
				},
			},
		},
		{
			To: []networkingv1.NetworkPolicyPeer{
				{
					NamespaceSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"name": "kube-system",
						},
					},
				},
			},
			Ports: []networkingv1.NetworkPolicyPort{
				{
					Protocol: ptr.To(corev1.ProtocolUDP),
					Port:     ptr.To(intstr.FromInt(53)),
				},
				{
					Protocol: ptr.To(corev1.ProtocolTCP),
					Port:     ptr.To(intstr.FromInt(53)),
				},
			},
		},
		{
			To: []networkingv1.NetworkPolicyPeer{
				{
					NamespaceSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"name": "k8s-mtp",
						},
					},
				},
			},
		},
	}

	if r.Config.TenantEgressPolicy == "internet" {
		egressRules = append(egressRules, networkingv1.NetworkPolicyEgressRule{
			To: []networkingv1.NetworkPolicyPeer{
				{
					IPBlock: &networkingv1.IPBlock{
						CIDR: "0.0.0.0/0",
					},
				},
			},
		})
	}

	return &networkingv1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "tenant-isolation",
			Namespace: ns,
			OwnerReferences: []metav1.OwnerReference{{
				APIVersion: v1.SchemeGroupVersion.String(),
				Kind:       "Tenant",
				Name:       t.Name,
				UID:        t.UID,
			}},
		},
		Spec: networkingv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{},
			PolicyTypes: []networkingv1.PolicyType{
				networkingv1.PolicyTypeIngress,
				networkingv1.PolicyTypeEgress,
			},
			Ingress: ingressRules,
			Egress:  egressRules,
		},
	}
}

func (r *TenantReconciler) buildLimitRange(t *v1.Tenant) *corev1.LimitRange {
	ns := fmt.Sprintf("tenant-%s-%s", t.Name, t.Spec.Name)

	var cpu, memory string
	switch t.Spec.Tier {
	case v1.TierFree:
		cpu = r.Config.LimitRangeDefaults.Free.DefaultCPU
		memory = r.Config.LimitRangeDefaults.Free.DefaultMemory
	case v1.TierPro:
		cpu = r.Config.LimitRangeDefaults.Pro.DefaultCPU
		memory = r.Config.LimitRangeDefaults.Pro.DefaultMemory
	case v1.TierEnterprise:
		cpu = r.Config.LimitRangeDefaults.Enterprise.DefaultCPU
		memory = r.Config.LimitRangeDefaults.Enterprise.DefaultMemory
	default:
		cpu = "100m"
		memory = "128Mi"
	}

	return &corev1.LimitRange{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "tenant-defaults",
			Namespace: ns,
			OwnerReferences: []metav1.OwnerReference{{
				APIVersion: v1.SchemeGroupVersion.String(),
				Kind:       "Tenant",
				Name:       t.Name,
				UID:        t.UID,
			}},
		},
		Spec: corev1.LimitRangeSpec{
			Limits: []corev1.LimitRangeItem{
				{
					Type: corev1.LimitTypeContainer,
					Default: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse(cpu),
						corev1.ResourceMemory: resource.MustParse(memory),
					},
					DefaultRequest: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse(cpu),
						corev1.ResourceMemory: resource.MustParse(memory),
					},
				},
			},
		},
	}
}
