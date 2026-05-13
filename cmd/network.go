package cmd

import (
	"context"
	"encoding/json"
	"fmt"

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

	defer writer.Close()

	for _, nad := range nadList.Items {
		cniType, vlanID := parseCNIConfig(nad.Spec.Config)
		writer.Write(&NetworkData{
			Name:      colorName(nad.Name),
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
	}
	return
}
