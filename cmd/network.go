package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	rcmd "github.com/rancher/cli/cmd"
	"github.com/urfave/cli/v2"
	k8smetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type NetworkData struct {
	Name      string
	Namespace string
	Type      string
	VLAN      string
}

type cniConfig struct {
	Type string `json:"type"`
	VLAN int    `json:"vlan"`
	// VLANTrunk carries the ranges of a trunked bridge network, where spec.config has "vlan": 0.
	VLANTrunk []cniVLANTrunkRange `json:"vlanTrunk,omitempty"`
}

type cniVLANTrunkRange struct {
	ID    int `json:"id,omitempty"`
	MinID int `json:"minID,omitempty"`
	MaxID int `json:"maxID,omitempty"`
}

// NetworkCommand defines the CLI command for listing VM networks
func NetworkCommand() *cli.Command {
	return &cli.Command{
		Name:    "network",
		Aliases: []string{"net"},
		Usage:   "Manipulate VM networks",
		Action:  networkList,
		Flags: []cli.Flag{
			&nsFlag,
		},
		Subcommands: cli.Commands{
			&cli.Command{
				Name:        "list",
				Aliases:     []string{"ls"},
				Usage:       "List VM networks",
				Description: "\nLists all NetworkAttachmentDefinitions available in Harvester",
				ArgsUsage:   "None",
				Action:      networkList,
				Flags: []cli.Flag{
					&nsFlag,
				},
			},
		},
	}
}

func networkList(ctx *cli.Context) (err error) {
	c, err := GetHarvesterClient(ctx)
	if err != nil {
		return
	}

	nadList, err := c.K8sCniCncfIoV1().NetworkAttachmentDefinitions(ctx.String("namespace")).List(context.TODO(), k8smetav1.ListOptions{})
	if err != nil {
		return
	}

	writer := rcmd.NewTableWriter([][]string{
		{"NAME", "Name"},
		{"NAMESPACE", "Namespace"},
		{"TYPE", "Type"},
		{"VLAN ID", "VLAN"},
	}, ctxv1)

	defer func() { _ = writer.Close() }()

	for _, nad := range nadList.Items {
		cniType, vlanID := parseCNIConfig(nad.Spec.Config)
		writer.Write(&NetworkData{
			Name:      nad.Name,
			Namespace: nad.Namespace,
			Type:      cniType,
			VLAN:      vlanID,
		})
	}

	return writer.Err()
}

// parseCNIConfig extracts the CNI type and VLAN ID from the raw CNI JSON config.
func parseCNIConfig(raw string) (cniType string, vlanID string) {
	var cfg cniConfig
	if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
		return "unknown", ""
	}
	cniType = cfg.Type
	if cfg.VLAN != 0 {
		vlanID = fmt.Sprintf("%d", cfg.VLAN)
		return
	}

	// A trunked network has no single VLAN ID, it carries a set of ranges instead.
	ranges := make([]string, 0, len(cfg.VLANTrunk))
	for _, trunk := range cfg.VLANTrunk {
		switch {
		case trunk.MinID != 0 && trunk.MaxID != 0 && trunk.MinID != trunk.MaxID:
			ranges = append(ranges, fmt.Sprintf("%d-%d", trunk.MinID, trunk.MaxID))
		case trunk.MinID != 0:
			ranges = append(ranges, fmt.Sprintf("%d", trunk.MinID))
		case trunk.ID != 0:
			ranges = append(ranges, fmt.Sprintf("%d", trunk.ID))
		}
	}
	if len(ranges) > 0 {
		vlanID = "trunk " + strings.Join(ranges, ",")
	}

	return
}
