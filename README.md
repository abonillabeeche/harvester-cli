[![Go Report Card](https://goreportcard.com/badge/github.com/abonillabeeche/harvester-cli)](https://goreportcard.com/report/github.com/abonillabeeche/harvester-cli)
[![Lint Go Code](https://github.com/abonillabeeche/harvester-cli/actions/workflows/lint.yml/badge.svg)](https://github.com/abonillabeeche/harvester-cli/actions/workflows/lint.yml)

# Harvester CLI

A fast, kubectl-style command-line tool for managing [Harvester HCI](https://harvesterhci.io) clusters — create and control VMs, images, volumes, networks, and hosts without leaving your terminal.

---

## Table of Contents

- [Features](#features)
- [Installation](#installation)
- [Configuration](#configuration)
- [Commands](#commands)
  - [VM Management](#vm-management)
  - [Images](#images)
  - [Image Catalog](#image-catalog)
  - [Networks](#networks)
  - [Volumes](#volumes)
  - [Hosts](#hosts)
  - [Templates](#templates)
  - [SSH Keypairs](#ssh-keypairs)
  - [Shell Access](#shell-access)
- [Dry-run and GitOps](#dry-run-and-gitops)
- [Tips and Gotchas](#tips-and-gotchas)

---

## Features

| Area | Capabilities |
|---|---|
| **Virtual Machines** | List, create, delete, start, stop, restart, live-migrate |
| **VM Images** | List (with StorageClass), upload from URL or file |
| **Image Catalog** | Curated list of cloud-init-enabled Linux images (Fedora, CentOS Stream, Debian, AlmaLinux, Rocky, Ubuntu, openSUSE); interactive picker, scriptable `create`, works offline via embedded JSON + `catalog init` cache |
| **Networks** | List NADs with VLAN info |
| **Volumes** | List PVCs with live Longhorn usage and StorageClass; create new PVCs |
| **Hosts** | List nodes with real-time CPU % and memory usage from the metrics API |
| **Templates** | List and inspect VM templates |
| **SSH Keypairs** | List registered public keys |
| **Shell** | Direct SSH into a running VM |
| **Config** | Login to Rancher and auto-download the Harvester kubeconfig |
| **Dry-run** | Print the Kubernetes YAML for any `create` command without applying it — ideal for GitOps workflows |

---

## Installation

### Homebrew (macOS and Linux — recommended)

```bash
brew install abonillabeeche/tap/harvester
```

Homebrew handles the macOS quarantine flag automatically, so Gatekeeper will not block the binary.

> **First-time setup:** add the tap once with `brew tap abonillabeeche/tap`, then install as above.

---

### Install script (macOS and Linux)

```bash
curl -fsSL https://raw.githubusercontent.com/abonillabeeche/harvester-cli/main/install.sh | sh
```

The script auto-detects your OS and architecture, downloads the right binary from the latest release, and removes the macOS quarantine flag. To install to a custom location:

```bash
INSTALL_DIR=~/.local/bin sh <(curl -fsSL https://raw.githubusercontent.com/abonillabeeche/harvester-cli/main/install.sh)
```

---

### Windows

Download the `.exe` for your architecture from the [latest release](https://github.com/abonillabeeche/harvester-cli/releases/latest) and place it somewhere on your `%PATH%`.

---

### Build from source

Requirements: Go 1.21+

```bash
git clone https://github.com/abonillabeeche/harvester-cli.git
cd harvester-cli
go build -o harvester .
sudo mv harvester /usr/local/bin/
harvester --version
```

---

## Configuration

`harvester` needs a kubeconfig that points to your Harvester cluster's Kubernetes API. There are three ways to provide it, in order of precedence:

### 1. Default file location (recommended)

Place the kubeconfig at `~/.harvester/config`. This is the path the CLI uses automatically with no extra flags.

```bash
mkdir -p ~/.harvester
cp /path/to/your/harvester-kubeconfig ~/.harvester/config
```

### 2. Environment variable

```bash
export HARVESTER_CONFIG=/path/to/kubeconfig
harvester vm list
```

### 3. Inline flag

```bash
harvester --harvester-config /path/to/kubeconfig vm list
```

### Automatic download via Rancher

If your Harvester cluster is imported into Rancher, you can download the kubeconfig automatically using a Rancher API token:

```bash
# Log in to your Rancher server
harvester login https://<RANCHER_URL> -t <RANCHER_API_TOKEN>

# Download the kubeconfig for the target Harvester cluster
harvester get-config <HARVESTER_CLUSTER_NAME>
# Saved to ~/.harvester/config automatically
```

---

## Commands

All commands accept `-n <namespace>` (or `--namespace`) to target a specific namespace, mirroring `kubectl` behavior. The default namespace is `default`.

---

### VM Management

```bash
harvester vm list [-n NAMESPACE]
```

Lists all VMs in the cluster with their state, CPU, memory, and IP address.

---

```bash
harvester vm create [FLAGS...] VM_NAME
```

Creates a VM. **Important: all flags must come before the VM name.**

| Flag | Short | Description |
|---|---|---|
| `--namespace` | `-n` | Target namespace (default: `default`) |
| `--cpus` | `-c` | Number of vCPUs (default: `1`) |
| `--memory` | `-m` | RAM size, e.g. `4Gi` (default: `1Gi`) |
| `--disk-size` | `-d` | Root disk size, e.g. `20Gi` (default: `10Gi`) |
| `--vm-image-id` | | Harvester image ID to boot from |
| `--ssh-keyname` | `-i` | SSH keypair name (use `namespace/name` for cross-namespace) |
| `--network` | | Network attachment, e.g. `default/vlan1` |
| `--template` | | VM template in `name:version` format |
| `--count` | | Create N identical VMs named `basename-1`…`basename-N` |
| `--user-data-filepath` | | Path to a cloud-init user-data YAML file |
| `--dry-run` | | Print the KubeVirt YAML without creating the VM |

Examples:

```bash
# Create a VM using a specific image and network
harvester vm create --cpus 2 --memory 4Gi --vm-image-id default/ubuntu-noble --network default/vlan1 --ssh-keyname default/mykey my-vm

# Create 3 VMs from a template
harvester vm create --template ubuntu-base:1 --count 3 test-vm

# Create a VM in a non-default namespace
harvester vm create -n dev --cpus 4 --memory 8Gi dev-vm

# Preview the manifest without applying it
harvester vm create --dry-run --vm-image-id default/ubuntu-noble --network default/vlan1 my-vm
```

---

```bash
harvester vm stop    VM_NAME [-n NAMESPACE]
harvester vm start   VM_NAME [-n NAMESPACE]
harvester vm restart VM_NAME [-n NAMESPACE]
harvester vm delete  VM_NAME [-n NAMESPACE]
```

---

```bash
harvester vm migrate [--node TARGET_NODE] VM_NAME [-n NAMESPACE]
```

Live-migrates a running VM to another host without downtime. Omit `--node` to let the scheduler choose the best target automatically.

```bash
# Migrate to a specific host
harvester vm migrate --namespace default --node gr6-2 my-vm

# Let the scheduler pick the target
harvester vm migrate -n default my-vm
```

---

### Images

```bash
harvester image list [-n NAMESPACE]
```

Lists VM images with their source type, StorageClass, and URL.

```
NAME              ID                     SOURCE TYPE   STORAGE CLASS   URL
ubuntu-noble      default/ubuntu-noble   download      tworeplicas     https://cloud-images.ubuntu.com/...
```

---

```bash
harvester image create --source URL [--storage-class CLASS] [-n NAMESPACE] IMAGE_NAME
```

Uploads a VM image from an HTTP/HTTPS URL (or local file path). Prints the image ID on success.

```bash
# Upload Ubuntu Noble using the tworeplicas storage class
harvester image create \
  --source https://cloud-images.ubuntu.com/noble/current/noble-server-cloudimg-amd64.img \
  --storage-class tworeplicas \
  ubuntu-noble
# Image created: default/ubuntu-noble

# Preview the manifest without applying it
harvester image create --dry-run \
  --source https://cloud-images.ubuntu.com/noble/current/noble-server-cloudimg-amd64.img \
  ubuntu-noble
```

---

### Image Catalog

The catalog is a curated list of publicly-available, **cloud-init-enabled** Linux images (Fedora, CentOS Stream, Debian, AlmaLinux, Rocky Linux, Ubuntu, openSUSE — see `image-metadata.json`). Instead of hunting for the right download URL for every cluster, you pick from the catalog and the CLI submits the image to Harvester.

The catalog metadata lives in `image-metadata.json` at the root of this repo. That same file is bundled into every CLI binary via `//go:embed`, and it can be cached to disk with `catalog init` — so the catalog works even when your workstation has no internet, as long as the Harvester servers themselves can reach the image URLs.

#### Where the catalog comes from

Every `catalog` subcommand resolves its metadata source in this order:

1. **`--metadata-url` flag** or **`HARVESTER_CATALOG_METADATA` env var**, if explicitly set (accepts `https://…`, `file://…`, or a plain filesystem path).
2. **`~/.harvester/image-metadata.json`** if it exists (the local cache written by `catalog init`).
3. **Default remote URL** (`https://raw.githubusercontent.com/abonillabeeche/harvester-cli/main/image-metadata.json`). If the HTTP fetch fails, the CLI transparently falls back to the copy **embedded in the binary** (with a warning), so `catalog` still works offline out of the box.

The interactive `catalog` command prints the resolved source at the top of its output so you can always tell where the list came from:

```
$ harvester image catalog
Image catalog source: /Users/you/.harvester/image-metadata.json
```

#### `catalog init` — cache the catalog for offline use

```bash
harvester image catalog init [--metadata-url URL] [--force]
```

Downloads the metadata JSON and writes it to `~/.harvester/image-metadata.json` (alongside the harvester CLI config). Subsequent `catalog` commands prefer this file automatically.

```bash
# First-time setup while online:
harvester image catalog init
# Downloading catalog from: https://raw.githubusercontent.com/abonillabeeche/harvester-cli/main/image-metadata.json
# Saving to:                /Users/you/.harvester/image-metadata.json
# Cached catalog with 10 OS group(s). Future 'image catalog' runs will use this file automatically.

# Refresh when new distros are added upstream:
harvester image catalog init --force

# Cache from a private mirror instead of GitHub:
harvester image catalog init --metadata-url https://mirror.internal/harvester/image-metadata.json --force
```

#### `catalog` — interactive picker

```bash
harvester image catalog [--namespace NS] [--storage-class SC] [--metadata-url URL]
```

Walks you through picking an OS group, then an image, then the namespace, then the StorageClass. If the caller has cluster-list permissions, namespace and StorageClass are shown as numbered pickers; otherwise the CLI falls back to a free-form text prompt (useful for Rancher-proxied Harvester kubeconfigs, which typically only grant namespaced reads).

```
$ harvester image catalog
Image catalog source: /Users/you/.harvester/image-metadata.json

NUMBER  NAME            NUMBER OF IMAGES
1       fedora          2
2       centos-stream   2
3       debian          2
4       almalinux       2
5       RockyLinux      37
6       ubuntu          6
7       leap            3
...
Insert a number to select the image OS:
1

NUMBER  NAME              VERSION  BUILD  URL
1       Fedora Cloud 43   43       1.6    https://download.fedoraproject.org/.../Fedora-Cloud-Base-Generic-43-1.6.x86_64.qcow2
2       Fedora Cloud 42   42       1.1    https://download.fedoraproject.org/.../Fedora-Cloud-Base-Generic-42-1.1.x86_64.qcow2

Insert a number to select an image to download:
1

Your image URL is : https://download.fedoraproject.org/.../Fedora-Cloud-Base-Generic-43-1.6.x86_64.qcow2

Enter the namespace to create the image in [default]: my-project

Enter the StorageClass name (empty = cluster default): tworeplicas

INFO Creating image "Fedora-Cloud-Base-Generic-43-1.6.x86_64.qcow2" in namespace "my-project" with StorageClass "tworeplicas"
INFO Image was created in Harvester with display name Fedora-Cloud-Base-Generic-43-1.6.x86_64.qcow2 and id image-a9b2f
```

Pass `--namespace` or `--storage-class` to skip the matching prompt.

#### `catalog list` — non-interactive listing

```bash
harvester image catalog list [OS] [--metadata-url URL]
```

Prints catalog entries without prompting. Optional `OS` filter limits output to a single group.

```bash
# Full catalog
harvester image catalog list

# Just Fedora
harvester image catalog list fedora
# VERSION  BUILD  SHORT NAME       URL
# 43       1.6    Fedora Cloud 43  https://download.fedoraproject.org/.../Fedora-Cloud-Base-Generic-43-1.6.x86_64.qcow2
# 42       1.1    Fedora Cloud 42  https://download.fedoraproject.org/.../Fedora-Cloud-Base-Generic-42-1.1.x86_64.qcow2
```

#### `catalog create` — scripted image creation

```bash
harvester image catalog create <OS>/<VERSION> \
  [--namespace NS] [--storage-class SC] [--display-name NAME] \
  [--description TEXT] [--dry-run] [--metadata-url URL]
```

Creates a VM image from a catalog entry by `<os>/<version>` selector (e.g. `fedora/43`, `ubuntu/24.04`). No prompts — flag values are used as-is. Empty `--storage-class` means "use the cluster default StorageClass". When multiple builds share the same version (e.g. Rocky's many `9.5-*` dated builds), the last entry in the catalog array wins (arrays are ordered oldest → newest) and the choice is logged.

```bash
# Create with cluster default StorageClass
harvester image catalog create fedora/43

# Explicit namespace + StorageClass + friendly display name
harvester image catalog create --namespace prod --storage-class tworeplicas \
  --display-name ubuntu-noble-base ubuntu/24.04

# Preview the YAML manifest without creating (GitOps-friendly)
harvester image catalog create --dry-run --storage-class tworeplicas debian/12

# Use a private mirror as the source
harvester image catalog create --metadata-url https://mirror.internal/catalog.json centos-stream/9
```

Error messages point you to what's available if you get the selector wrong:

```
$ harvester image catalog create ubuntu/99
FATA no image with version "99" for ubuntu. Available versions: 18.04, 20.04, 22.04, 24.04, 24.10, 25.04

$ harvester image catalog create nonexistent/1.0
FATA unknown OS "nonexistent". Available: RockyLinux, almalinux, centos-stream, debian, fedora, leap, leap15.4+, microos, tumbleweed, ubuntu
```

> **Flag ordering:** because of a urfave/cli v2 quirk, flags must appear **before** the positional selector: `create --dry-run ubuntu/24.04` — not `create ubuntu/24.04 --dry-run`.

---

### Networks

```bash
harvester network list [-n NAMESPACE]
```

Lists NetworkAttachmentDefinitions with their CNI type and VLAN ID.

```
NAME    NAMESPACE   TYPE     VLAN ID
vlan1   default     bridge   1
vlan10  default     bridge   10
```

---

### Volumes

```bash
harvester volume list [-n NAMESPACE]
```

Lists PersistentVolumeClaims cross-referenced with Longhorn to show actual used space.

```
NAME         NAMESPACE   STATE    CAPACITY   USED       STORAGE CLASS
my-vm-disk   default     Healthy  20.0 GiB   3.2 GiB    tworeplicas
data-vol     default     Healthy  100.0 GiB  45.1 GiB   harvester-longhorn
```

---

```bash
harvester volume create --storage-class CLASS --size SIZE [-n NAMESPACE] [--dry-run] VOLUME_NAME
```

Creates a PersistentVolumeClaim backed by the specified StorageClass. Use binary suffixes for size (`Gi`, `Mi`).

| Flag | Short | Description |
|---|---|---|
| `--storage-class` | `--sc` | StorageClass for the volume (see `harvester volume list-storageclass`) |
| `--size` | `-s` | Volume size, e.g. `10Gi`, `500Mi` |
| `--namespace` | `-n` | Target namespace (default: `default`) |
| `--dry-run` | | Print the PVC YAML without creating it |

```bash
harvester volume create --sc tworeplicas --size 20Gi my-data-vol
# Volume created: default/my-data-vol
```

---

```bash
harvester volume list-storageclass
```

Lists all StorageClasses in the cluster — equivalent to `kubectl get sc`.

```
NAME                  PROVISIONER             RECLAIM POLICY   BINDING MODE    ALLOW EXPANSION
harvester-longhorn    driver.longhorn.io      Delete           Immediate       true
tworeplicas           driver.longhorn.io      Delete           Immediate       true
```

---

### Hosts

```bash
harvester host list
```

Lists all cluster nodes with real-time CPU and memory usage pulled from the Kubernetes Metrics API. Falls back gracefully to `<unknown>` if the metrics server is unavailable.

```
NAME    STATUS  ROLES                      AGE    CPU(cores)  CPU%  MEM USE    MEM%  MEM TOTAL
gr6-1   Ready   etcd,control-plane,master  769d   1918m       12%   26.0 GiB   41%   62.2 GiB
gr6-2   Ready   etcd                       769d   1126m       10%   3.0 GiB    10%   28.3 GiB
gr6-3   Ready   etcd,master,control-plane  769d   2480m       16%   26.5 GiB   42%   62.2 GiB
```

- **CPU(cores)**: live usage in millicores (e.g. `1918m` = 1.918 cores)
- **CPU%**: usage relative to allocatable CPU on that node
- **MEM USE**: live memory usage from the metrics API
- **MEM%**: usage relative to allocatable memory on that node
- **MEM TOTAL**: total installed RAM (`node.Status.Capacity`)

---

### Templates

```bash
harvester template list [-n NAMESPACE]

# Show a specific template version
harvester template show NAME:VERSION [-n NAMESPACE]
harvester template show ubuntu-base:1
```

---

### SSH Keypairs

```bash
harvester keypair list [-n NAMESPACE]
```

Lists SSH public keys registered in Harvester, which can be referenced by name when creating VMs.

---

### Shell Access

```bash
harvester shell [--ssh-user USER] [--ssh-key PATH] VM_NAME
```

Opens an interactive SSH session directly into a running VM. Requires `ssh` to be available on the local system.

| Flag | Default | Description |
|---|---|---|
| `--ssh-user` | `ubuntu` | Username to connect as |
| `--ssh-key` | `~/.ssh/id_rsa` | Path to private key |
| `--ssh-port` | `22` | SSH port |

```bash
harvester shell --ssh-user ubuntu --ssh-key ~/.ssh/mykey my-vm
```

---

## Dry-run and GitOps

Every `create` subcommand accepts a `--dry-run` flag. Instead of calling the Kubernetes API, the CLI prints the fully-rendered YAML manifest to stdout and exits. This is useful for:

- **GitOps workflows** — generate manifests locally, commit them to a Git repo, and let Fleet, Flux or ArgoCD apply them to the cluster.
- **Reviewing changes before applying** — inspect the exact object the CLI would create before committing to it.
- **Piping into `kubectl apply`** — run `harvester vm create --dry-run ... | kubectl apply -f -` for one-shot creation using the same flags as your normal workflow.

### Volume

```bash
harvester volume create --dry-run --sc tworeplicas --size 5Gi testvol1
```

```yaml
---
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
    creationTimestamp: null
    name: testvol1
    namespace: default
spec:
    accessModes:
        - ReadWriteMany
    resources:
        requests:
            storage: 5Gi
    storageClassName: tworeplicas
    volumeMode: Block
status: {}
```

### Image

```bash
harvester image create --dry-run \
  --source https://cloud-images.ubuntu.com/noble/current/noble-server-cloudimg-amd64.img \
  --storage-class tworeplicas \
  ubuntu-noble
```

```yaml
---
apiVersion: harvesterhci.io/v1beta1
kind: VirtualMachineImage
metadata:
    creationTimestamp: null
    generateName: image-
    namespace: default
spec:
    displayName: ubuntu-noble
    sourceType: download
    storageClassParameters: {}
    targetStorageClassName: tworeplicas
    url: https://cloud-images.ubuntu.com/noble/current/noble-server-cloudimg-amd64.img
status:
    progress: 0
```

### VM

```bash
harvester vm create --dry-run \
  --vm-image-id default/image-abc12 \
  --network default/vlan1 \
  --cpus 2 --memory 4Gi --disk-size 20Gi \
  my-vm
```

```yaml
---
apiVersion: kubevirt.io/v1
kind: VirtualMachine
metadata:
    annotations:
        harvesterhci.io/volumeClaimTemplates: '[{"metadata":{"name":"my-vm-disk-0-...","annotations":{"harvesterhci.io/imageId":"default/image-abc12"}},"spec":{"accessModes":["ReadWriteMany"],"resources":{"requests":{"storage":"20Gi"}},"volumeMode":"Block","storageClassName":"longhorn-image-abc12"}}]'
        networks.harvesterhci.io/ips: '[]'
    labels:
        harvesterhci.io/creator: harvester
    name: my-vm
    namespace: default
spec:
    runStrategy: Always
    template:
        spec:
            domain:
                cpu:
                    cores: 2
                    sockets: 1
                    threads: 1
                devices:
                    disks:
                        - disk:
                            bus: virtio
                          name: disk-0
                        - disk:
                            bus: virtio
                          name: cloudinitdisk
                    interfaces:
                        - bridge: {}
                          model: virtio
                          name: nic-1
                resources:
                    limits:
                        cpu: "2"
                        memory: 4Gi
            networks:
                - multus:
                    networkName: default/vlan1
                  name: nic-1
            volumes:
                - name: disk-0
                  persistentVolumeClaim:
                    claimName: my-vm-disk-0-...
                - cloudInitNoCloud:
                    networkData: "..."
                    userData: "#cloud-config\n..."
                  name: cloudinitdisk
```

### GitOps example

```bash
# Generate manifests for a new environment
harvester volume create --dry-run --sc tworeplicas --size 50Gi -n prod db-vol     > manifests/db-vol.yaml
harvester image create  --dry-run --source https://example.com/os.qcow2 prod-os   >> manifests/images.yaml
harvester vm create     --dry-run --vm-image-id default/prod-os \
                          --network default/vlan10 --cpus 4 --memory 8Gi -n prod \
                          db-server                                                > manifests/db-server.yaml

# Commit and push — let your GitOps controller apply them
git add manifests/ && git commit -m "add prod db-server" && git push
```

---

## Tips and Gotchas

### Flag ordering with `vm create`

Due to how Go's flag parser works with positional arguments, **all flags must appear before the VM name**:

```bash
# Correct
harvester vm create --cpus 2 --memory 4Gi my-vm

# Wrong — flags after the name are silently ignored
harvester vm create my-vm --cpus 2 --memory 4Gi
```

### Cross-namespace references

When your VM lives in namespace `dev` but the SSH key or network is in `default`, use the `namespace/name` format:

```bash
harvester vm create -n dev \
  --ssh-keyname default/mykey \
  --network default/vlan1 \
  dev-vm
```

### Template version is required for `template show`

The `show` subcommand requires both name and version separated by a colon:

```bash
harvester template show ubuntu-base:1
```

### Debug mode

Add `--debug` before any command to enable verbose logging:

```bash
harvester --debug vm list
```
