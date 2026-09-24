# v0.5.0 — Harvester v1.9.0 support

Built and tested against **Harvester v1.9.0** (KubeVirt 1.8.x). Earlier 1.x clusters keep working,
minus the two 1.9.0-only features below.

## Harvester v1.9.0

- **`image create --backend backingimage|cdi`** — v1.9.0 adds `VirtualMachineImage.spec.backend`.
  `cdi` is the only backend that can place an image on a third-party CSI StorageClass (Ceph RBD,
  LINSTOR, Longhorn v2). Omit the flag and it is inferred from the StorageClass provisioner.
  `image list` gained a `BACKEND` column.
- **`import --source-cluster-type ova`** — the new `OvaSource` type, for pulling OVA files off an
  HTTP server. Credentials are optional, and `--http-timeout` maps to
  `OvaSourceOptions.HttpTimeoutSeconds` (default 600).

## New commands

- `harvester image delete [-n NS] NAME...` — by display name or resource name, namespace-scoped.
- `harvester volume delete [-n NS] [--force] NAME...` — refuses a PVC that a VM still references
  unless `--force`, since that delete otherwise sits in `Terminating`.

## Fixes

- `--ssh-keyname` is finally written into cloud-init; VMs used to boot with no way to log in.
- `image create --storage-class` was ignored on the backingimage path, and failed the create
  outright on clusters whose default StorageClass carries only the deprecated beta annotation.
- `import list` no longer hardcodes `harvester-system`; it lists every namespace, takes `-n`, and
  prints a `NAMESPACE` column.
- `import create` read `--mapping` and `--source-cluster-name`, neither of which exists, so network
  mappings and the source were always empty. Both `import create` and `import source-add` now set
  `metadata.namespace`, which the API server requires to match the request URL.
- Flags given after a positional argument are rejected instead of silently dropped — a stray
  `harvester vm create my-vm --dry-run` used to create the VM for real.
- An unknown subcommand ran the parent's list action and exited 0; it now errors and lists the
  valid subcommands.
- `--memory` and `--disk-size` are validated up front: a bare number is bytes, and a malformed
  value used to panic inside `resource.MustParse`.
- `vm start`/`vm stop` take several names, `vm restart` accepts `--vm-name`, `template show`
  degrades gracefully when the template's image is gone, `volume list` formats capacity
  consistently, and `network list` shows VLAN trunk ranges.
- `volume list-storageclass` gained a `DEFAULT` column, and calls out a class marked default only
  with the deprecated beta annotation — Harvester ignores it and later fails backing-image creates
  with `no default storageClass found`.
- The `import` subcommands explain the 404 they get when the `vm-import-controller` addon is
  disabled, instead of just reporting that the resource does not exist.
- `golangci-lint run` goes from 23 issues to 0.

## Verification

Every command was exercised against three live Harvester v1.9.0 clusters, covering Ceph RBD,
Longhorn v1 and Longhorn v2: image creation on both backends, VM create from image and template,
SSH into the booted guest, live migration between hosts, the full OVA import flow, and the new
delete paths.
