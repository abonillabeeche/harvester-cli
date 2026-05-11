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
  - [Networks](#networks)
  - [Volumes](#volumes)
  - [Hosts](#hosts)
  - [Templates](#templates)
  - [SSH Keypairs](#ssh-keypairs)
  - [Shell Access](#shell-access)
- [Tips and Gotchas](#tips-and-gotchas)

---

## Features

| Area | Capabilities |
|---|---|
| **Virtual Machines** | List, create, delete, start, stop, restart, live-migrate |
| **VM Images** | List (with StorageClass), upload from URL or file |
| **Networks** | List NADs with VLAN info |
| **Volumes** | List PVCs with live Longhorn usage and StorageClass |
| **Hosts** | List nodes with real-time CPU % and memory usage from the metrics API |
| **Templates** | List and inspect VM templates |
| **SSH Keypairs** | List registered public keys |
| **Shell** | Direct SSH into a running VM |
| **Config** | Login to Rancher and auto-download the Harvester kubeconfig |

---

## Installation

### Build from source

Requirements: Go 1.21+

```bash
git clone https://github.com/abonillabeeche/harvester-cli.git
cd harvester-cli

# macOS arm64
CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -o harvester .

# macOS x86_64
CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -o harvester .

# Linux x86_64
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o harvester .
```

Move the binary somewhere on your `$PATH`:

```bash
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

Examples:

```bash
# Create a VM using a specific image and network
harvester vm create --cpus 2 --memory 4Gi --vm-image-id default/ubuntu-noble --network default/vlan1 --ssh-keyname default/mykey my-vm

# Create 3 VMs from a template
harvester vm create --template ubuntu-base:1 --count 3 test-vm

# Create a VM in a non-default namespace
harvester vm create -n dev --cpus 4 --memory 8Gi dev-vm
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
```

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
