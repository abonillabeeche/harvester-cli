package cmd

import (
	"context"
	"fmt"

	rcmd "github.com/rancher/cli/cmd"
	"github.com/urfave/cli/v2"
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
	defer writer.Close()

	for _, pvc := range pvcList.Items {
		capacity := ""
		if q, ok := pvc.Spec.Resources.Requests["storage"]; ok {
			capacity = q.String()
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
			Name:         colorName(pvc.Name),
			Namespace:    pvc.Namespace,
			State:        colorStatus(state),
			Capacity:     capacity,
			Used:         used,
			StorageClass: sc,
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
