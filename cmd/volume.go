package cmd

import (
	"context"
	"fmt"

	rcmd "github.com/rancher/cli/cmd"
	"github.com/urfave/cli/v2"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	k8smetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type VolumeData struct {
	Name         string
	Namespace    string
	State        string
	Capacity     string
	Used         string
	StorageClass string
}

type StorageClassData struct {
	Name              string
	Default           string
	Provisioner       string
	ReclaimPolicy     string
	VolumeBindingMode string
	AllowExpansion    string
}

const (
	defaultStorageClassAnnotation     = "storageclass.kubernetes.io/is-default-class"
	betaDefaultStorageClassAnnotation = "storageclass.beta.kubernetes.io/is-default-class"
)

// defaultMarker says whether a StorageClass is the cluster default. Harvester only honours the
// modern annotation, so a class carrying nothing but the deprecated beta one is called out: it looks
// default to kubectl, but Harvester behaves as if the cluster has no default at all and rejects
// backing-image creates with "no default storageClass found for backingImage".
func defaultMarker(annotations map[string]string) string {
	if annotations[defaultStorageClassAnnotation] == "true" {
		return "*"
	}
	if annotations[betaDefaultStorageClassAnnotation] == "true" {
		return "(beta only, Harvester ignores it)"
	}
	return ""
}

func VolumeCommand() *cli.Command {
	return &cli.Command{
		Name:    "volume",
		Aliases: []string{"vol"},
		Usage:   "Manipulate Harvester volumes",
		Action:  volumeList,
		Flags: []cli.Flag{
			&nsFlag,
		},
		Subcommands: cli.Commands{
			&cli.Command{
				Name:        "list",
				Aliases:     []string{"ls"},
				Usage:       "List volumes",
				Description: "\nLists all PersistentVolumeClaims with Longhorn usage stats",
				ArgsUsage:   "None",
				Action:      volumeList,
				Flags: []cli.Flag{
					&nsFlag,
				},
			},
			&cli.Command{
				Name:        "create",
				Aliases:     []string{"c"},
				Usage:       "Create a volume (PersistentVolumeClaim)",
				Description: "\nCreates a PersistentVolumeClaim backed by the specified StorageClass",
				ArgsUsage:   "VOLUME_NAME",
				Action:      volumeCreate,
				Flags: []cli.Flag{
					&nsFlag,
					&cli.StringFlag{
						Name:     "storage-class",
						Aliases:  []string{"sc"},
						Usage:    "StorageClass name for the volume (see 'harvester volume list-storageclass')",
						EnvVars:  []string{"HARVESTER_VOLUME_SC"},
						Required: true,
					},
					&cli.StringFlag{
						Name:     "size",
						Aliases:  []string{"s"},
						Usage:    "Volume size using binary suffixes: 10Gi, 20Gi, 500Mi (Gi = gibibytes, Mi = mebibytes). Plain G/M are decimal and not recommended.",
						EnvVars:  []string{"HARVESTER_VOLUME_SIZE"},
						Required: true,
					},
					&cli.BoolFlag{
						Name:  "dry-run",
						Usage: "Print the YAML manifest that would be submitted without creating the resource",
					},
				},
			},
			&cli.Command{
				Name:        "delete",
				Aliases:     []string{"del", "rm"},
				Usage:       "Delete one or more volumes (PersistentVolumeClaims)",
				Description: "\nDeletes PersistentVolumeClaims. A volume still referenced by a VM is refused unless --force is given",
				ArgsUsage:   "VOLUME_NAME [VOLUME_NAME...]",
				Action:      volumeDelete,
				Flags: []cli.Flag{
					&nsFlag,
					&cli.BoolFlag{
						Name:  "force",
						Usage: "Delete the volume even if a VM still references it",
					},
				},
			},
			&cli.Command{
				Name:        "list-storageclass",
				Aliases:     []string{"ls-sc", "storageclass"},
				Usage:       "List available StorageClasses",
				Description: "\nLists all StorageClasses in the cluster (equivalent to kubectl get sc)",
				ArgsUsage:   "None",
				Action:      volumeListStorageClass,
			},
		},
	}
}

func volumeList(ctx *cli.Context) error {
	kube, err := GetKubeClient(ctx)
	if err != nil {
		return err
	}

	harv, err := GetHarvesterClient(ctx)
	if err != nil {
		return err
	}

	pvcList, err := kube.CoreV1().PersistentVolumeClaims(ctx.String("namespace")).List(context.TODO(), k8smetav1.ListOptions{})
	if err != nil {
		return err
	}

	// Build map of Longhorn volume name → ActualSize for used-space lookup.
	// Longhorn volumes live in longhorn-system; their names match the PV name.
	type longhornInfo struct {
		actualSize int64
		state      string
	}
	lhMap := map[string]longhornInfo{}
	lhVols, err := harv.LonghornV1beta2().Volumes("longhorn-system").List(context.TODO(), k8smetav1.ListOptions{})
	if err == nil {
		for _, v := range lhVols.Items {
			lhMap[v.Name] = longhornInfo{
				actualSize: v.Status.ActualSize,
				state:      string(v.Status.State),
			}
		}
	}

	writer := rcmd.NewTableWriter([][]string{
		{"NAME", "Name"},
		{"NAMESPACE", "Namespace"},
		{"STATE", "State"},
		{"CAPACITY", "Capacity"},
		{"USED", "Used"},
		{"STORAGE CLASS", "StorageClass"},
	}, ctxv1)
	defer func() { _ = writer.Close() }()

	for _, pvc := range pvcList.Items {
		capacity := ""
		if q, ok := pvc.Spec.Resources.Requests["storage"]; ok {
			// Quantity.String() replays whatever format the PVC was written with, so a VM disk
			// created through the Harvester UI prints as "34359738368" next to a "12Gi" from the CLI.
			capacity = formatBytes(q.Value())
		}

		state := string(pvc.Status.Phase)
		used := ""
		if lh, ok := lhMap[pvc.Spec.VolumeName]; ok {
			state = lh.state
			if lh.actualSize > 0 {
				used = formatBytes(lh.actualSize)
			}
		}

		sc := ""
		if pvc.Spec.StorageClassName != nil {
			sc = *pvc.Spec.StorageClassName
		}

		writer.Write(&VolumeData{
			Name:         pvc.Name,
			Namespace:    pvc.Namespace,
			State:        state,
			Capacity:     capacity,
			Used:         used,
			StorageClass: sc,
		})
	}

	return writer.Err()
}

func volumeCreate(ctx *cli.Context) error {
	if ctx.NArg() != 1 {
		return fmt.Errorf("expected exactly one argument: VOLUME_NAME")
	}

	volName := ctx.Args().First()
	sc := ctx.String("storage-class")
	blockMode := corev1.PersistentVolumeBlock

	qty, err := resource.ParseQuantity(ctx.String("size"))
	if err != nil {
		return fmt.Errorf("invalid size %q: %w", ctx.String("size"), err)
	}

	pvc := &corev1.PersistentVolumeClaim{
		TypeMeta: k8smetav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "PersistentVolumeClaim",
		},
		ObjectMeta: k8smetav1.ObjectMeta{
			Name:      volName,
			Namespace: ctx.String("namespace"),
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteMany},
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: qty,
				},
			},
			StorageClassName: &sc,
			VolumeMode:       &blockMode,
		},
	}

	if ctx.Bool("dry-run") {
		out, err := toYAML(pvc)
		if err != nil {
			return fmt.Errorf("dry-run: %w", err)
		}
		fmt.Printf("---\n%s", out)
		return nil
	}

	kube, err := GetKubeClient(ctx)
	if err != nil {
		return err
	}

	created, err := kube.CoreV1().PersistentVolumeClaims(ctx.String("namespace")).Create(context.TODO(), pvc, k8smetav1.CreateOptions{})
	if err != nil {
		return err
	}

	fmt.Printf("Volume created: %s/%s\n", created.Namespace, created.Name)
	return nil
}

func volumeDelete(ctx *cli.Context) error {
	if ctx.NArg() == 0 {
		return fmt.Errorf("expected at least one argument: VOLUME_NAME")
	}

	kube, err := GetKubeClient(ctx)
	if err != nil {
		return err
	}

	namespace := ctx.String("namespace")

	// A PVC that a VM still lists as a volume deletes into Terminating and sits there until the VM
	// goes away, which looks like a hang. Name the VM instead.
	inUseBy := map[string]string{}
	if !ctx.Bool("force") {
		harv, err := GetHarvesterClient(ctx)
		if err != nil {
			return err
		}
		vmList, err := harv.KubevirtV1().VirtualMachines(namespace).List(context.TODO(), k8smetav1.ListOptions{})
		if err != nil {
			return err
		}
		for _, vm := range vmList.Items {
			if vm.Spec.Template == nil {
				continue
			}
			for _, vol := range vm.Spec.Template.Spec.Volumes {
				if vol.PersistentVolumeClaim != nil {
					inUseBy[vol.PersistentVolumeClaim.ClaimName] = vm.Name
				}
			}
		}
	}

	for _, volName := range ctx.Args().Slice() {
		if vmName, used := inUseBy[volName]; used {
			return fmt.Errorf("volume %s/%s is still used by VM %s, delete the VM first or pass --force", namespace, volName, vmName)
		}

		if err := kube.CoreV1().PersistentVolumeClaims(namespace).Delete(context.TODO(), volName, k8smetav1.DeleteOptions{}); err != nil {
			return fmt.Errorf("volume %s/%s could not be deleted: %w", namespace, volName, err)
		}
		fmt.Printf("Volume deleted: %s/%s\n", namespace, volName)
	}

	return nil
}

func volumeListStorageClass(ctx *cli.Context) error {
	kube, err := GetKubeClient(ctx)
	if err != nil {
		return err
	}

	scList, err := kube.StorageV1().StorageClasses().List(context.TODO(), k8smetav1.ListOptions{})
	if err != nil {
		return err
	}

	writer := rcmd.NewTableWriter([][]string{
		{"NAME", "Name"},
		{"DEFAULT", "Default"},
		{"PROVISIONER", "Provisioner"},
		{"RECLAIM POLICY", "ReclaimPolicy"},
		{"BINDING MODE", "VolumeBindingMode"},
		{"ALLOW EXPANSION", "AllowExpansion"},
	}, ctxv1)
	defer func() { _ = writer.Close() }()

	for _, sc := range scList.Items {
		rp := "Delete"
		if sc.ReclaimPolicy != nil {
			rp = string(*sc.ReclaimPolicy)
		}
		bm := "Immediate"
		if sc.VolumeBindingMode != nil {
			bm = string(*sc.VolumeBindingMode)
		}
		ae := "false"
		if sc.AllowVolumeExpansion != nil && *sc.AllowVolumeExpansion {
			ae = "true"
		}
		writer.Write(&StorageClassData{
			Name:              sc.Name,
			Default:           defaultMarker(sc.Annotations),
			Provisioner:       sc.Provisioner,
			ReclaimPolicy:     rp,
			VolumeBindingMode: bm,
			AllowExpansion:    ae,
		})
	}

	return writer.Err()
}

func formatBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(b)/float64(div), "KMGTPE"[exp])
}
