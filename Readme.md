# guardit

Admission controller ecosystem for your homelab Kubernetes cluster. Catches bad deployments before they reach the cluster - or rejects them at the gate if they try to bypass.

```
# deployit catches it before the image is even built
deployit deploy ./my-api --host api.yourdomain.com --replicas 10

-> guardit: checking policy (~/guardit.yaml)
✗ guardit: deployment rejected by policy
  [ReplicaLimit] replica count 10 is too high for this cluster (max 5)

# kubectl can't bypass it either
kubectl apply -f overkill.yaml
Error from server: [ReplicaLimit] replica count 10 is too high for this cluster (max 5)
```

### How it works

```
deployit deploy                      kubectl apply
       │                                   │
  guardit SDK                     Kubernetes API Server
  (local check)                            │
       │                        ValidatingWebhookConfiguration
  rejected or                              │
  continues                        POST /validate
                                           │
                                    guardit webhook
                                    (runs on the Pi)
                                           │
                                    policy engine
                                           │
                                  allowed / denied
```

Two enforcement points. The SDK check in `deployit` is fast - pure local CPU, no network - and stops you before wasting time building and pushing an image that would be rejected anyway. The webhook is the hard stop for anything that reaches the cluster directly.

### Architecture

| Part                       | Location    | Role                                                                 |
|----------------------------|-------------|----------------------------------------------------------------------|
| Webhook (`cmd/guardit`)    | On the Pi   | The enforcer. An HTTPS server Kubernetes calls on every deployment.  |
| CLI (`cmd/guard-cli`)      | Your laptop | The linter. Check a manifest against your policy before applying it. |
| SDK (`pkg/sdk`)            | Library     | The contract. What `deployit` imports to run checks locally.         |

### Installation

```bash
git clone https://github.com/gappylul/guardit
cd guardit
go build -o guard-cli ./cmd/guard-cli
sudo mv guard-cli /usr/local/bin/guard-cli
```

**or**

```bash
go install github.com/gappylul/guardit/cmd/guard-cli@latest
```

### Setup

#### 1. Create your policy file

Drop a `guardit.yaml` in your home directory or project root. The SDK and CLI auto-discover it - no flags needed.

```yaml
apiVersion: guardit.gappy.hu/v1
kind: Policy
metadata:
  name: homelab
spec:
  # Maximum replicas any deployment may request. 0 = unlimited.
  replicaLimit: 5

  # Image must be prefixed with one of these. Empty = any registry allowed.
  allowedRegistries:
    - ghcr.io/gappylul

  # These label keys must be present on every pod template.
  requiredLabels:
    - app

  # Every container must declare cpu and memory limits.
  requireResourceLimits: false
```

You can also define the policy directly in Go - same struct, no YAML required:

```go
p := &policy.Policy{
    APIVersion: "guardit.gappy.hu/v1",
    Kind:       "Policy",
    Metadata:   policy.Metadata{Name: "homelab"},
    Spec: policy.Spec{
        ReplicaLimit:      5,
        AllowedRegistries: []string{"ghcr.io/gappylul"},
        RequiredLabels:    []string{"app"},
    },
}
```

#### 2. Deploy the webhook to your cluster

```bash
# Generate a TLS cert for the webhook service
openssl req -x509 -newkey rsa:4096 -nodes \
  -keyout tls.key -out tls.crt -days 3650 \
  -subj "/CN=guardit.guardit.svc" \
  -addext "subjectAltName=DNS:guardit.guardit.svc,DNS:guardit.guardit.svc.cluster.local"

kubectl create secret tls guardit-tls \
  --cert=tls.crt --key=tls.key -n guardit

# Inject the CA bundle and apply
CA_BUNDLE=$(base64 < tls.crt | tr -d '\n')
sed -i "s|caBundle: \"\"|caBundle: \"$CA_BUNDLE\"|" deploy/webhookconfig.yaml

kubectl apply -f deploy/webhook.yaml
kubectl apply -f deploy/webhookconfig.yaml
```

Update the policy any time by editing the `guardit-policy` ConfigMap - no redeploy needed.

```bash
kubectl edit configmap guardit-policy -n guardit
```

#### 3. Wire into deployit

Add one block to `deployit/cmd/deploy.go` before `BuildAndPush`:

```go
fmt.Printf("-> guardit: checking policy (%s)\n", guarditSDK.PolicySource())
result, err := guarditSDK.Check(guarditPolicy.DeploymentRequest{
    Name:     name,
    Image:    fmt.Sprintf("%s/%s:preflight", registry, name),
    Replicas: replicas,
    Labels:   map[string]string{"app": name},
    Containers: []guarditPolicy.ContainerRequest{
        {Name: name, ResourceLimitsDeclared: true},
    },
})
if err != nil {
    return fmt.Errorf("guardit: %w", err)
}
if !result.Allowed {
    fmt.Println(guarditSDK.FormatViolations(result))
    return fmt.Errorf("deployment rejected by guardit policy")
}
fmt.Println("-> guardit: ✓ policy checks passed")
```

### Usage

#### Lint a manifest locally

```bash
guard-cli check deployment.yaml
-> policy: homelab
✓ my-api passed all policy checks

guard-cli check deployment.yaml --policy ./guardit.yaml
✗ my-api failed policy checks:
  [ReplicaLimit] replica count 10 is too high for this cluster (max 5)
  [MissingLabels] "my-api" is missing required pod template labels: app
```

#### Policy discovery order

The SDK and CLI look for a policy in this order, stopping at the first match:

1. `$GUARDIT_POLICY` env var (explicit path)
2. `guardit.yaml` / `guardit.yml` in the current directory
3. `guardit.yaml` / `guardit.yml` in `$HOME`
4. Built-in defaults (replicaLimit: 5, requiredLabels: [app])

### Rules

| Code                    | What it checks                                     | Config key              |
|-------------------------|----------------------------------------------------|-------------------------|
| `ReplicaLimit`          | Replica count must not exceed the cap              | `replicaLimit`          |
| `RegistryNotAllowed`    | Image must be prefixed by an allowed registry      | `allowedRegistries`     |
| `MissingLabels`         | Pod template must have all required label keys     | `requiredLabels`        |
| `MissingResourceLimits` | Every container must declare cpu and memory limits | `requireResourceLimits` |

Rules with zero/empty/false config are skipped entirely - guardit is opt-in per rule.

### Adding a new rule

Two steps. The engine picks it up automatically.

```go
// 1. Add the field to Spec in internal/policy/types.go
type Spec struct {
    // ...existing fields...
    DenyLatestTag bool `json:"denyLatestTag"`
}

// 2. Write the rule and register it in internal/policy/rules.go
func ruleNoLatestTag(spec Spec, req DeploymentRequest) *Violation {
    if !spec.DenyLatestTag {
        return nil
    }
    if strings.HasSuffix(req.Image, ":latest") || !strings.Contains(req.Image, ":") {
        return &Violation{
            Code:    "LatestTagForbidden",
            Message: fmt.Sprintf("image %q must use a specific tag, not :latest", req.Image),
        }
    }
    return nil
}

var allRules = []ruleFunc{
    // ...existing rules...
    ruleNoLatestTag,
}
```

The webhook, CLI, and SDK all enforce it with no further changes.

### Config

#### Webhook environment variables

| Env var          | Default         | Description                                                                 |
|------------------|-----------------|-----------------------------------------------------------------------------|
| `GUARDIT_POLICY` | (auto-discover) | Explicit path to a guardit.yaml                                             |
| `TLS_CERT_FILE`  | (none)          | Path to TLS certificate. If unset, runs plain HTTP on :8080 with a warning. |
| `TLS_KEY_FILE`   | (none)          | Path to TLS private key                                                     |

#### SDK environment variables

| Env var          | Default         | Description                     |
|------------------|-----------------|---------------------------------|
| `GUARDIT_POLICY` | (auto-discover) | Explicit path to a guardit.yaml |

### Part of the homelab ecosystem

guardit is one piece of a larger self-hosted platform:

- **[deployit](https://github.com/gappylul/deployit)** - deploy any project to your cluster with one command
- **[flagd](https://github.com/gappylul/flagd)** - feature flag server, Redis-backed, zero config
- **[terraform-provider-flagd](https://github.com/gappylul/terraform-provider-flagd)** - manage feature flags as code
- **[guardit](https://github.com/gappylul/guardit)** - policy enforcement so none of the above can misbehave

Recommended stack: Raspberry Pi 5, k3s, Traefik, Cloudflare Tunnel, Tailscale.