# Reducing the memory footprint of rbac-permissions-operator

**Jira:** ROSAENG-1296
**Branch:** `ROSAENG-1296-reduce-memory-footprint`
**Goal:** Reduce the operator's memory footprint on large clusters **without changing
any externally observable behavior**, so this change can be reviewed as a pure net
improvement.

---

## TL;DR

- The operator's resident memory grows **linearly** with the total number of RBAC
  objects it caches — dominated by a **cluster-wide RoleBinding informer** that caches
  *every* RoleBinding in *every* namespace, even though the operator only ever asks
  "does this one RoleBinding exist?".
- We **disable caching for `RoleBinding`** using the official controller-runtime
  `client.CacheOptions.DisableFor` option. RoleBinding existence checks become targeted
  `Get` calls to the API server instead of being served from (and requiring) a
  full-cluster cache.
- We **stop the Namespace controller from listing every namespace and every
  RoleBinding** on each reconcile. It reconciles a single namespace, so it now matches
  that one namespace's name against each permission's regex directly.
- We remove a redundant full copy of the `NamespaceList` in the SubjectPermission
  controller and hoist per-iteration `regexp.MustCompile` calls out of hot loops.
- All of the above are **behavior-neutral**: the same RoleBindings/ClusterRoleBindings
  are created, the same status conditions are set, and the same allow/deny regex
  semantics apply. Only the *mechanism* (cached list scans → targeted reads) changes.
- We keep **`ClusterRole`, `ClusterRoleBinding`, and `Namespace` fully cached**, on
  purpose (see "Why not disable those caches too").

---

## Background: how the operator uses memory

The operator runs two controllers:

1. **SubjectPermission controller** — for each SubjectPermission CR, creates
   ClusterRoleBindings (cluster permissions) and RoleBindings in every namespace that
   matches the permission's `namespacesAllowedRegex` / `namespacesDeniedRegex`.
2. **Namespace controller** — when a namespace is created, creates the RoleBindings
   that the matching SubjectPermissions call for.

Both use a controller-runtime **cached client**. A cached client backs each object type
it reads with an **informer**: a cluster-wide LIST + WATCH that keeps a full in-memory
copy of every object of that type, kept up to date by the watch stream.

### The memory model is linear, not exponential

Empirically (reproduced on a test cluster):

| RoleBindings in cluster | Operator RSS |
|------------------------:|-------------:|
| ~0 (empty namespaces)   | ~35 Mi       |
| ~46,300                 | ~260 Mi      |

That works out to roughly **~1.4 KB of resident memory per cached RoleBinding**. The
relationship is linear:

```
mem ≈ base
    + Σ (cached_objects_of_type × bytes_per_object)   # informer caches
    + (concurrent_reconciles × NamespaceList_copy_size) # transient per-reconcile
```

Nested loops in the reconcilers (`permissions × namespaces`) cost **CPU and allocation
rate**, not resident memory — they don't retain anything. So the way to cut memory is to
cut what we cache, not to restructure the loops.

### Why this matters for ROSA

The `dedicated-admins` and `dedicated-admin-serviceaccounts` SubjectPermissions use
`namespacesAllowedRegex: .*`. That means the operator legitimately creates a RoleBinding
in nearly **every** namespace on the cluster. Its working set — and therefore the
RoleBinding informer — scales with the size of the cluster. On a large cluster this is
the single largest consumer of the operator's memory, and it is pure overhead: the
operator never needs the whole set in memory at once.

---

## What changed

### 1. Disable the RoleBinding cache (`main.go`) — the big win

```go
Client: client.Options{
    Cache: &client.CacheOptions{
        DisableFor: []client.Object{
            &rbacv1.RoleBinding{},
        },
    },
},
```

`DisableFor` is an **official controller-runtime feature**. Reads of the listed types
are routed straight to the API server instead of being served from (and requiring) an
informer. No cluster-wide RoleBinding informer is ever built, so the largest persistent
cache disappears.

This is safe because the operator's only interaction with existing RoleBindings is an
**existence check before create**. It never lists RoleBindings to make a decision about
the set as a whole, and it never watches RoleBindings to trigger reconciliation.

### 2. Existence check via targeted `Get` (`subjectpermission_controller.go`)

Previously, for each permission the controller did a namespace-scoped `List` of
RoleBindings and scanned the result by name. That list was served by the cluster-wide
informer. Now it does a direct, targeted read for the exact object:

```go
existing := &v1.RoleBinding{}
getErr := r.Get(ctx, client.ObjectKey{Namespace: ns, Name: roleBinding.Name}, existing)
if getErr == nil {
    continue // already present
}
if !k8serr.IsNotFound(getErr) {
    return ctrl.Result{}, fmt.Errorf(...) // real error, requeue
}
err := r.Create(ctx, roleBinding)
if err != nil {
    if k8serr.IsAlreadyExists(err) {
        continue // race-safe: someone created it between Get and Create
    }
    return ctrl.Result{}, err
}
```

`Create` remains the authoritative, race-safe check — `IsAlreadyExists` is tolerated
exactly as before. Behavior is identical: existing binding → skip; missing → create.

### 3. Namespace controller: single-namespace match (`namespace_controller.go`)

The Namespace controller reconciles **one** namespace (the one in the request). It used
to `List` *all* namespaces and *all* RoleBindings in the target namespace anyway. Both
lists are removed. It now:

- lists only SubjectPermissions (few, small, and the objects we match against), and
- matches the single reconciled namespace's name directly against each permission's
  allow/deny regex via a new helper:

```go
if controllerutil.NamespaceMatchesPermission(instance.Name,
       permission.NamespacesAllowedRegex, permission.NamespacesDeniedRegex) &&
   controllerutil.ValidateNamespace(instance) {
    // create RoleBinding
}
```

`NamespaceMatchesPermission` applies the **same allow-then-deny semantics** as
`GenerateSafeList`, just for a single name instead of a whole list. A unit test asserts
that, for every namespace in a list, the single-name match agrees with
`GenerateSafeList` membership — i.e. it is provably behavior-preserving.

**Behavior-preservation detail:** on `IsAlreadyExists` from `Create`, we do **not** set
`bindingCreated = true`. This mirrors the previous behavior where an already-existing
binding was skipped without triggering a SubjectPermission status update (avoiding an
unnecessary reconcile of the other controller).

### 4. Drop the redundant NamespaceList copy (`subjectpermission_controller.go`)

The controller used to build a *second* full `NamespaceList` to hold the
validated/non-terminating namespaces. On a large cluster that is a second full copy of
every namespace object, allocated on every reconcile. It now filters in place, reusing
the original backing array:

```go
newNsList := corev1.NamespaceList{}
filtered := nsList.Items[:0]
for i := range nsList.Items {
    if controllerutil.ValidateNamespace(&nsList.Items[i]) {
        filtered = append(filtered, nsList.Items[i])
    } else {
        reqLogger.Info(...)
    }
}
newNsList.Items = filtered
```

### 5. Hoist `regexp.MustCompile` out of per-namespace loops (`controllerutil.go`)

`allowedNamespacesList` and `safeListAfterDeniedRegex` compiled the regex **once per
namespace** — tens of thousands of compilations per reconcile on a large cluster. The
compile is now done once before the loop. This is a CPU/allocation-rate improvement; the
matching result is unchanged.

---

## Why not disable those caches too? (ClusterRole / ClusterRoleBinding / Namespace)

We deliberately keep these **fully cached**. Disabling their caches would trade one
memory problem for a worse one:

- **Uncached reads re-allocate on every reconcile.** The SubjectPermission controller
  lists all ClusterRoles, all ClusterRoleBindings, and all Namespaces. If those were
  uncached, each reconcile would pull the entire list from the API server and allocate
  it fresh — a large, repeated **transient spike**. A memory spike can trigger OOM just
  as surely as a large steady cache, and it also hammers the API server.
- **The informer mechanism exists precisely to avoid that.** A cache amortizes the cost:
  one LIST+WATCH, kept warm, shared across reconciles.
- **RoleBinding is different**, and that difference is why it's the one type we disable:
  its cached set is by far the largest (one per namespace × permissions, i.e. it scales
  with the cluster), *and* we only ever need one object at a time. Small, bounded,
  point-read access pattern → cache is pure overhead. The others are read as whole lists
  and are much smaller, so the cache is a net win.

### What about a metadata-only cache?

`cache.Options.ByObject` with a `Transform` that strips `managedFields`, or
`PartialObjectMetadata` reads, can shrink cached objects. These were considered and
**rejected** for this change:

- The `managedFields`-stripping transform is not an official, first-class "metadata-only
  cache" feature — it's a hand-rolled trick, and it's easy to get subtly wrong.
- `PartialObjectMetadata` would require reworking every read site and its tests.

Both add complexity and reviewer burden for a change we want to be obviously correct and
behavior-neutral. The RoleBinding `DisableFor` change already removes the dominant cost
using a supported API. If further reduction is needed later, a metadata-only cache for
the remaining types can be evaluated as a separate, scoped change.

### Should we just raise the memory limit?

For very large clusters the limit may still need a bump — the working set is genuinely
proportional to cluster size. But raising the limit *without* this change means paying
for the RoleBinding informer, which is avoidable overhead. Recommendation: **land this
change first**, then size the limit against the reduced, mostly-fixed footprint.

---

## Behavior-neutrality checklist

| Concern | Before | After | Same? |
|---|---|---|---|
| Which RoleBindings get created | matched by allow/deny regex over full ns list | same regex, matched per-namespace | ✅ |
| Which ClusterRoleBindings get created | from `ClusterPermissions` | unchanged | ✅ |
| Existence-before-create semantics | list+scan, then create | get, then create; `AlreadyExists` tolerated | ✅ |
| Status conditions set | on actual create only | on actual create only | ✅ |
| Allow-then-deny precedence | deny wins | deny wins | ✅ |
| Terminating/nonexistent ns filtered | yes | yes (in place) | ✅ |

Verified by `go build ./...`, `go vet ./...`, and `go test ./...` (all controller and
util suites pass, including the new `NamespaceMatchesPermission` equivalence test).

---

## Deferred correctness concerns (out of scope — tracked separately)

Two pre-existing correctness issues surfaced during this investigation. They are **not**
addressed here (they would change behavior) and are tracked in a separate Jira card:

1. **RoleBindings leak when a SubjectPermission is deleted.** The finalizer cleanup only
   deletes a Prometheus metric; the created RoleBindings/ClusterRoleBindings have no
   `ownerReference` and no managed-by label, so nothing deletes them.
2. **Manual edits to a managed RoleBinding are not reconciled back.** The existence check
   is name-only, so if someone edits a RoleBinding's contents (keeping the name), the
   operator never corrects it.

See the companion Jira card for details.
