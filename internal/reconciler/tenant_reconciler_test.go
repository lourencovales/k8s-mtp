package reconciler

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"git.assilvestrar.club/lourenco/k8s-mtp/internal/config"
	v1 "git.assilvestrar.club/lourenco/k8s-mtp/pkg/api/v1"
)

func TestCreateOrUpdateNamespace(t *testing.T) {
	r, client := newTestReconciler(t)
	tenant := newTestTenant(v1.TierFree)

	ns := r.buildNamespace(tenant)
	if err := r.createOrUpdate(context.Background(), ns); err != nil {
		t.Fatal(err)
	}

	var created corev1.Namespace
	if err := client.Get(context.Background(), types.NamespacedName{Name: ns.Name}, &created); err != nil {
		t.Fatal(err)
	}
	if created.Name != "tenant-my-test-tenant-ajax" {
		t.Errorf("Name is wrong, expected 'tenant-my-test-tenant-ajax', got %s", created.Name)
	}
	if created.Labels["tenant.k8s-mtp.io/managed"] != "true" {
		t.Errorf("Label is wrong, expected 'tenant.k8s-mtp.io/managed'")
	}
	if len(created.OwnerReferences) != 1 || created.OwnerReferences[0].Name != "my-test-tenant" {
		t.Errorf("Owner Reference is wrong, expected 'my-test-tenant', got %s", created.OwnerReferences[0].Name)
	}
}

func TestCreateOrUpdateResourceQuota(t *testing.T) {
	r, client := newTestReconciler(t)
	tenantFree := newTestTenant(v1.TierFree)
	tenantPro := newTestTenant(v1.TierPro)
	tenantEnterprise := newTestTenant(v1.TierEnterprise)

	rqFree := r.buildResourceQuota(tenantFree)
	rqPro := r.buildResourceQuota(tenantPro)
	rqEnterprise := r.buildResourceQuota(tenantEnterprise)

	if err := r.createOrUpdate(context.Background(), rqFree); err != nil {
		t.Fatal(err)
	}
	if err := r.createOrUpdate(context.Background(), rqPro); err != nil {
		t.Fatal(err)
	}
	if err := r.createOrUpdate(context.Background(), rqEnterprise); err != nil {
		t.Fatal(err)
	}

	var createdFree corev1.ResourceQuota
	var createdPro corev1.ResourceQuota
	var createdEnterprise corev1.ResourceQuota

	if err := client.Get(context.Background(),
		types.NamespacedName{Name: rqFree.Name, Namespace: rqFree.Namespace},
		&createdFree); err != nil {
		t.Fatal(err)
	}
	if err := client.Get(context.Background(),
		types.NamespacedName{Name: rqPro.Name, Namespace: rqPro.Namespace},
		&createdPro); err != nil {
		t.Fatal(err)
	}
	if err := client.Get(context.Background(),
		types.NamespacedName{Name: rqEnterprise.Name, Namespace: rqEnterprise.Namespace},
		&createdEnterprise); err != nil {
		t.Fatal(err)
	}

	cpuFree := createdFree.Spec.Hard[corev1.ResourceCPU]
	memFree := createdFree.Spec.Hard[corev1.ResourceMemory]
	podsFree := createdFree.Spec.Hard[corev1.ResourcePods]
	storageFree := createdFree.Spec.Hard[corev1.ResourceRequestsStorage]
	if cpuFree.String() != "500m" ||
		memFree.String() != "1Gi" ||
		podsFree.String() != "20" ||
		storageFree.String() != "10Gi" {
		t.Errorf("Expected spec differs from the created spec on free tier")
	}

	cpuPro := createdPro.Spec.Hard[corev1.ResourceCPU]
	memPro := createdPro.Spec.Hard[corev1.ResourceMemory]
	podsPro := createdPro.Spec.Hard[corev1.ResourcePods]
	storagePro := createdPro.Spec.Hard[corev1.ResourceRequestsStorage]
	if cpuPro.String() != "4" ||
		memPro.String() != "8Gi" ||
		podsPro.String() != "100" ||
		storagePro.String() != "100Gi" {
		t.Errorf("Expected spec differs from the created spec on pro tier")
	}

	cpuEnterprise := createdEnterprise.Spec.Hard[corev1.ResourceCPU]
	memEnterprise := createdEnterprise.Spec.Hard[corev1.ResourceMemory]
	podsEnterprise := createdEnterprise.Spec.Hard[corev1.ResourcePods]
	storageEnterprise := createdEnterprise.Spec.Hard[corev1.ResourceRequestsStorage]
	if cpuEnterprise.String() != "8" ||
		memEnterprise.String() != "16Gi" ||
		podsEnterprise.String() != "200" ||
		storageEnterprise.String() != "200Gi" {
		t.Errorf("Expected spec differs from the created spec on enterprise tier")
	}
}

func TestCreateOrUpdateNetworkPolicy(t *testing.T) {
	r, client := newTestReconciler(t)
	tenant := newTestTenant(v1.TierFree)

	np := r.buildNetworkPolicy(tenant)

	if err := r.createOrUpdate(context.Background(), np); err != nil {
		t.Error(err)
	}

	var created networkingv1.NetworkPolicy
	if err := client.Get(context.Background(),
		types.NamespacedName{Name: np.Name, Namespace: np.Namespace},
		&created); err != nil {
		t.Error(err)
	}

	ingress := created.Spec.Ingress
	egress := created.Spec.Egress
	if len(ingress) != 2 {
		t.Fatalf("expected 2 ingress rules, got %d", len(ingress))
	}
	if len(egress) != 3 {
		t.Fatalf("expected 3 egress rules, got %d", len(egress))
	}
	if ingress[0].From[0].PodSelector == nil {
		t.Errorf("expected PodSelector on the first ingress rule")
	}
	if ingress[1].From[0].NamespaceSelector == nil ||
		ingress[1].From[0].NamespaceSelector.MatchLabels["name"] != "k8s-mtp" {
		t.Errorf("expeced NamespaceSelector with name=k8s-mtp on second ingress rule")
	}
	if egress[1].Ports[0].Port.IntValue() != 53 {
		t.Errorf("expected DNS port 53 on second egress rule")
	}
}

func TestCreateOrUpdateRole(t *testing.T) {
	r, client := newTestReconciler(t)
	tenant := newTestTenant(v1.TierFree)

	ar := buildAdminRole(tenant)
	or := buildOperatorRole(tenant)

	if err := r.createOrUpdate(context.Background(), ar); err != nil {
		t.Fatal(err)
	}
	if err := r.createOrUpdate(context.Background(), or); err != nil {
		t.Fatal(err)
	}

	var createdAr rbacv1.Role
	var createdOr rbacv1.Role
	if err := client.Get(context.Background(),
		types.NamespacedName{Name: ar.Name, Namespace: ar.Namespace},
		&createdAr); err != nil {
		t.Fatal(err)
	}
	if err := client.Get(context.Background(),
		types.NamespacedName{Name: or.Name, Namespace: or.Namespace},
		&createdOr); err != nil {
		t.Fatal(err)
	}

	roleAr := createdAr.Rules
	if len(roleAr) != 5 {
		t.Fatalf("expected 5 admin role rules, got %d", len(roleAr))
	}
	if len(roleAr[0].Resources) < 1 || roleAr[0].Resources[0] != "pods" {
		t.Errorf("expected first resource to be 'pods', got %v", roleAr[0].Resources)
	}
	roleOr := createdOr.Rules
	if len(roleOr) != 3 {
		t.Fatalf("expected 3 operator role rules, got %d", len(roleOr))
	}
	if len(roleOr[0].Resources) < 1 || roleOr[0].Resources[0] != "pods" {
		t.Errorf("expected first resource to be 'pods', got %v", roleOr[0].Resources)
	}
}

func TestCreateOrUpdateRoleBinding(t *testing.T) {
	r, client := newTestReconciler(t)
	tenant := newTestTenant(v1.TierFree)

	arb := buildAdminRoleBinding(tenant)
	orb := buildOperatorRoleBinding(tenant)

	if err := r.createOrUpdate(context.Background(), arb); err != nil {
		t.Fatal(err)
	}
	if err := r.createOrUpdate(context.Background(), orb); err != nil {
		t.Fatal(err)
	}

	var createdArb rbacv1.RoleBinding
	var createdOrb rbacv1.RoleBinding
	if err := client.Get(context.Background(),
		types.NamespacedName{Name: arb.Name, Namespace: arb.Namespace},
		&createdArb); err != nil {
		t.Fatal(err)
	}
	if err := client.Get(context.Background(),
		types.NamespacedName{Name: orb.Name, Namespace: orb.Namespace},
		&createdOrb); err != nil {
		t.Fatal(err)
	}

	if len(createdArb.Subjects) != 1 {
		t.Fatalf("expected 1 rule binding subjects, got %d", len(createdArb.Subjects))
	}
	if createdArb.RoleRef.Name != "tenant-my-test-tenant-admin" {
		t.Errorf("expected RoleRef Name field to be %s, got %s", "tenant-my-test-tenant-admin", createdArb.RoleRef.Name)
	}
	if len(createdOrb.Subjects) != 1 {
		t.Fatalf("expected 1 rule binding subjects, got %d", len(createdOrb.Subjects))
	}
	if createdOrb.RoleRef.Name != "tenant-my-test-tenant-operator" {
		t.Errorf("expected RoleRef Name field to be %s, got %s", "tenant-my-test-tenant-operator", createdOrb.RoleRef.Name)
	}
}

func TestCreateOrUpdateLimitRange(t *testing.T) {
	r, client := newTestReconciler(t)
	tenant := newTestTenant(v1.TierFree)

	lr := r.buildLimitRange(tenant)

	if err := r.createOrUpdate(context.Background(), lr); err != nil {
		t.Fatal(err)
	}

	var created corev1.LimitRange

	if err := client.Get(context.Background(),
		types.NamespacedName{Name: lr.Name, Namespace: lr.Namespace},
		&created); err != nil {
		t.Fatal(err)
	}

	lrSpec := created.Spec.Limits[0].Default
	cpu := lrSpec[corev1.ResourceCPU]
	mem := lrSpec[corev1.ResourceMemory]
	if cpu.Cmp(resource.MustParse("100m")) != 0 {
		t.Errorf("Expected spec differs from the created spec on free tier")
	}
	if mem.Cmp(resource.MustParse("128Mi")) != 0 {
		t.Errorf("Expected spec differs from the created spec on free tier")
	}
}

func newTestReconciler(t *testing.T) (*TenantReconciler, client.Client) {
	t.Helper()
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	_ = networkingv1.AddToScheme(scheme)
	_ = rbacv1.AddToScheme(scheme)
	_ = v1.AddToScheme(scheme)

	cfg := config.Config{}
	cfg.LimitRangeDefaults.Free.DefaultCPU = "100m"
	cfg.LimitRangeDefaults.Free.DefaultMemory = "128Mi"

	cl := fake.NewClientBuilder().WithScheme(scheme).Build()
	r := &TenantReconciler{Client: cl, Config: &cfg}
	return r, cl
}

func newTestTenant(tier v1.Tier) *v1.Tenant {
	return &v1.Tenant{
		ObjectMeta: metav1.ObjectMeta{Name: "my-test-tenant", UID: types.UID("test-uid")},
		Spec:       v1.TenantSpec{Name: "ajax", Tier: tier},
	}
}
