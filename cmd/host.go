package cmd

import (
	"context"
	"fmt"
	"strings"
	"time"

	rcmd "github.com/rancher/cli/cmd"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
	corev1 "k8s.io/api/core/v1"
	k8smetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	metricsv1 "k8s.io/metrics/pkg/client/clientset/versioned"
)

// HostData holds one row of the host table.
type HostData struct {
	Name     string
	Status   string
	Roles    string
	Age      string
	CPU      string
	CPUPct   string
	MemUse   string
	MemPct   string
	MemTotal string
}

// hostUsage holds raw CPU (millicores) and memory (bytes) usage for a node.
type hostUsage struct {
	cpu    int64
	memory int64
}

func HostCommand() *cli.Command {
	return &cli.Command{
		Name:    "host",
		Aliases: []string{"node"},
		Usage:   "Manage Harvester hosts",
		Action:  hostList,
		Subcommands: cli.Commands{
			&cli.Command{
				Name:        "list",
				Aliases:     []string{"ls"},
				Usage:       "List hosts with CPU and memory usage",
				Description: "\nLists all Harvester nodes with resource usage from the Kubernetes metrics API",
				ArgsUsage:   "None",
				Action:      hostList,
			},
		},
	}
}

func hostList(ctx *cli.Context) error {
	kube, err := GetKubeClient(ctx)
	if err != nil {
		return err
	}

	nodes, err := kube.CoreV1().Nodes().List(context.TODO(), k8smetav1.ListOptions{})
	if err != nil {
		return err
	}

	// Fetch per-node usage from the metrics server; degrade gracefully if absent.
	usageMap := map[string]hostUsage{}
	if restCfg, err := GetRESTClientAndConfig(ctx); err == nil {
		if mc, err := metricsv1.NewForConfig(restCfg); err == nil {
			if nmList, err := mc.MetricsV1beta1().NodeMetricses().List(context.TODO(), k8smetav1.ListOptions{}); err != nil {
				logrus.Debugf("metrics API unavailable: %v", err)
			} else {
				for _, nm := range nmList.Items {
					usageMap[nm.Name] = hostUsage{
						cpu:    nm.Usage.Cpu().MilliValue(),
						memory: nm.Usage.Memory().Value(),
					}
				}
			}
		}
	}

	writer := rcmd.NewTableWriter([][]string{
		{"NAME", "Name"},
		{"STATUS", "Status"},
		{"ROLES", "Roles"},
		{"AGE", "Age"},
		{"CPU(cores)", "CPU"},
		{"CPU%", "CPUPct"},
		{"MEM USE", "MemUse"},
		{"MEM%", "MemPct"},
		{"MEM TOTAL", "MemTotal"},
	}, ctxv1)
	defer writer.Close()

	for i := range nodes.Items {
		node := &nodes.Items[i]
		u := usageMap[node.Name]
		writer.Write(&HostData{
			Name:     colorName(node.Name),
			Status:   colorStatus(hostNodeStatus(node)),
			Roles:    hostNodeRoles(node),
			Age:      hostNodeAge(node.CreationTimestamp.Time),
			CPU:      hostCPUCores(u),
			CPUPct:   colorPercent(hostCPUPercent(node, u)),
			MemUse:   hostMemBytes(u),
			MemPct:   colorPercent(hostMemPercent(node, u)),
			MemTotal: hostMemCapacity(node),
		})
	}

	return writer.Err()
}

func hostNodeStatus(node *corev1.Node) string {
	for _, c := range node.Status.Conditions {
		if c.Type == corev1.NodeReady {
			if c.Status == corev1.ConditionTrue {
				return "Ready"
			}
			return "NotReady"
		}
	}
	return "Unknown"
}

func hostNodeRoles(node *corev1.Node) string {
	var roles []string
	for label := range node.Labels {
		if strings.HasPrefix(label, "node-role.kubernetes.io/") {
			roles = append(roles, strings.TrimPrefix(label, "node-role.kubernetes.io/"))
		}
	}
	if len(roles) == 0 {
		return "<none>"
	}
	return strings.Join(roles, ",")
}

func hostNodeAge(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd", int(d.Hours()/24))
	}
}

func hostCPUCores(u hostUsage) string {
	if u.cpu == 0 && u.memory == 0 {
		return "<unknown>"
	}
	return fmt.Sprintf("%dm", u.cpu)
}

func hostCPUPercent(node *corev1.Node, u hostUsage) string {
	if u.cpu == 0 && u.memory == 0 {
		return "<unknown>"
	}
	alloc, ok := node.Status.Allocatable[corev1.ResourceCPU]
	if !ok || alloc.MilliValue() == 0 {
		return "<unknown>"
	}
	return fmt.Sprintf("%d%%", u.cpu*100/alloc.MilliValue())
}

func hostMemBytes(u hostUsage) string {
	if u.cpu == 0 && u.memory == 0 {
		return "<unknown>"
	}
	return formatBytes(u.memory)
}

func hostMemPercent(node *corev1.Node, u hostUsage) string {
	if u.cpu == 0 && u.memory == 0 {
		return "<unknown>"
	}
	alloc, ok := node.Status.Allocatable[corev1.ResourceMemory]
	if !ok || alloc.Value() == 0 {
		return "<unknown>"
	}
	return fmt.Sprintf("%d%%", u.memory*100/alloc.Value())
}

func hostMemCapacity(node *corev1.Node) string {
	cap, ok := node.Status.Capacity[corev1.ResourceMemory]
	if !ok {
		return "<unknown>"
	}
	return formatBytes(cap.Value())
}
