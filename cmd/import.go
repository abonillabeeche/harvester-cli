package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	VMImportV1 "github.com/harvester/vm-import-controller/pkg/apis/migration.harvesterhci.io/v1beta1"
	rcmd "github.com/rancher/cli/cmd"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var (
	harvesterMigrationAPIGroup = "migration.harvesterhci.io/v1beta1"
)

// sourceClusterTypes maps the value of --source-cluster-type to the API resource that backs it and
// the kind a VirtualMachineImport has to reference. Harvester v1.9.0 adds "ova" to the two source
// types that existed before.
var sourceClusterTypes = map[string]struct {
	resource string
	kind     string
}{
	"vmware":    {resource: "vmwaresources", kind: "VmwareSource"},
	"openstack": {resource: "openstacksources", kind: "OpenstackSource"},
	"ova":       {resource: "ovasources", kind: "OvaSource"},
}

const sourceClusterTypeUsage = "Type of the source cluster to import the VM from, this can take values of 'vmware', 'openstack' or 'ova'"

// resolveSourceClusterType looks up the API resource and kind for a --source-cluster-type value.
func resolveSourceClusterType(sourceType string) (resource string, kind string, err error) {
	entry, found := sourceClusterTypes[sourceType]
	if !found {
		return "", "", fmt.Errorf("invalid source cluster type: %v, must be \"openstack\", \"vmware\" or \"ova\"", sourceType)
	}
	return entry.resource, entry.kind, nil
}

type VMImportData struct {
	Name          string
	Namespace     string
	VMName        string
	SourceCluster string
	ClusterType   string
	Status        string
}

// Manages VM imports using vm-import-controller of Harvester
func ImportCommand() *cli.Command {
	return &cli.Command{
		Name:  "import",
		Usage: "Manage VM imports",
		Subcommands: []*cli.Command{
			importEnableCommand(),
			importListCommand(),
			importCreateCommand(),
			importDeleteCommand(),
			importSourceAddCommand(),
			importSourceDeleteCommand(),
		},
	}
}

func importEnableCommand() *cli.Command {
	return &cli.Command{
		Name:   "enable",
		Usage:  "Enable VM import",
		Action: enableVMImport,
	}
}

func importListCommand() *cli.Command {
	return &cli.Command{
		Name:   "list",
		Usage:  "List VM imports",
		Action: listVMImports,
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "namespace",
				Aliases: []string{"n"},
				Usage:   "Namespace to list VM imports from, defaults to all namespaces",
			},
		},
	}
}

func importCreateCommand() *cli.Command {
	return &cli.Command{
		Name:      "create",
		Usage:     "Create VM import",
		ArgsUsage: "VM_IMPORT_NAME",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "vm-name",
				Usage:    "Name of the VM to import on target infrastructure",
				Required: true,
				EnvVars:  []string{"HARVESTER_IMPORT_VM_NAME"},
			},
			&cli.StringSliceFlag{
				Name:    "network-mapping",
				Usage:   "Network mapping for the imported VM in the format  <source-network-name>:<target-network-name>.",
				Aliases: []string{"net-map"},
			},
			&cli.StringFlag{
				Name:     "source-cluster",
				Usage:    "Name of the source cluster to import the VM from",
				Required: true,
				EnvVars:  []string{"HARVESTER_IMPORT_SOURCE_CLUSTER"},
			},
			&cli.StringFlag{
				Name:     "source-cluster-type",
				Usage:    sourceClusterTypeUsage,
				Required: true,
				EnvVars:  []string{"HARVESTER_IMPORT_SOURCE_CLUSTER_TYPE"},
				Value:    "vmware",
			},
			&cli.StringFlag{
				Name:     "source-cluster-namespace",
				Aliases:  []string{"namespace", "n"},
				Usage:    "Namespace of the InfrastructureSource to be used for source cluster configuration",
				Required: true,
				EnvVars:  []string{"HARVESTER_IMPORT_SOURCE_CLUSTER_NAMESPACE"},
			},
		},
		Action: createVMImport,
	}
}

func importSourceAddCommand() *cli.Command {
	return &cli.Command{
		Name:      "source-add",
		Usage:     "Add a source cluster for VM import",
		ArgsUsage: "VM_IMPORT_CLUSTER_NAME",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "source-cluster-type",
				Usage:    sourceClusterTypeUsage,
				Required: true,
				EnvVars:  []string{"HARVESTER_IMPORT_SOURCE_CLUSTER_TYPE"},
				Value:    "vmware",
			},
			&cli.StringFlag{
				Name:     "source-cluster-namespace",
				Aliases:  []string{"namespace", "n"},
				Usage:    "Namespace of the InfrastructureSource to be used for source cluster configuration",
				Required: true,
				EnvVars:  []string{"HARVESTER_IMPORT_SOURCE_CLUSTER_NAMESPACE"},
			},
			&cli.StringFlag{
				Name:     "endpoint",
				Usage:    "Endpoint of the source cluster, for the 'ova' type this is the URL the OVA files are served from",
				Required: true,
				EnvVars:  []string{"HARVESTER_IMPORT_SOURCE_CLUSTER_ENDPOINT"},
			},
			&cli.IntFlag{
				Name:    "http-timeout",
				Usage:   "Download timeout in seconds for the 'ova' source type, 0 means no timeout",
				EnvVars: []string{"HARVESTER_IMPORT_SOURCE_CLUSTER_HTTP_TIMEOUT"},
				Value:   VMImportV1.DefaultHttpTimeoutSeconds,
			},
			&cli.StringFlag{
				Name:     "dc",
				Usage:    "Datacenter of the source cluster",
				Required: false,
				EnvVars:  []string{"HARVESTER_IMPORT_SOURCE_CLUSTER_DC"},
			},
			&cli.StringFlag{
				Name:     "region",
				Usage:    "Region of the source cluster",
				Required: false,
				EnvVars:  []string{"HARVESTER_IMPORT_SOURCE_CLUSTER_REGION"},
			},
			&cli.StringFlag{
				Name: "credentials-secret",
				// Optional only for the 'ova' type, which can read from an unauthenticated server.
				// configureVMImport rejects a missing secret for the other types.
				Usage:   "Reference to the secret containing credentials for the source cluster in the format: <namespace>/<secret-name>. Optional for the 'ova' source type",
				EnvVars: []string{"HARVESTER_IMPORT_SOURCE_CLUSTER_CREDENTIALS_SECRET"},
			},
		},
		Action: configureVMImport,
	}
}

func importSourceDeleteCommand() *cli.Command {
	return &cli.Command{
		Name:      "source-delete",
		Usage:     "Delete a source cluster for VM import",
		ArgsUsage: "VM_IMPORT_CLUSTER_NAME",
		Action:    deleteVMImportSource,
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name: "namespace",
				// source-add spells this --source-cluster-namespace, accept both here.
				Aliases: []string{"n", "source-cluster-namespace"},
				Usage:   "Namespace of the source cluster to be deleted",
				EnvVars: []string{"HARVESTER_IMPORT_SOURCE_CLUSTER_NAMESPACE"},
			},
			&cli.StringFlag{
				Name:    "source-cluster-type",
				Usage:   sourceClusterTypeUsage,
				Aliases: []string{"type"},
				EnvVars: []string{"HARVESTER_IMPORT_SOURCE_CLUSTER_TYPE"},
				Value:   "vmware",
			},
		},
	}
}

func importDeleteCommand() *cli.Command {
	return &cli.Command{
		Name:      "delete",
		Usage:     "Delete VM import",
		ArgsUsage: "VM_IMPORT_NAME",
		Action:    deleteVMImport,
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "namespace",
				Aliases: []string{"n"},
				Usage:   "Namespace of the VM import to be deleted",
			},
		},
	}
}

func enableVMImport(ctx *cli.Context) error {
	c, err := GetHarvesterClient(ctx)

	if err != nil {
		return err
	}

	vmImportAddon, err := c.HarvesterhciV1beta1().Addons("harvester-system").Get(context.Background(), "vm-import-controller", v1.GetOptions{})

	if err != nil {
		return fmt.Errorf("failed to get vm-import-controller Addon resource in Harvester: %v", err)
	}

	if vmImportAddon.Spec.Enabled {
		return nil
	}

	vmImportEnabled, err := c.HarvesterhciV1beta1().Addons("harvester-system").Patch(context.TODO(), "vm-import-controller", types.MergePatchType, []byte(`{"spec":{"enabled":true}}`), v1.PatchOptions{})

	if err != nil || !vmImportEnabled.Spec.Enabled {
		return fmt.Errorf("failed to enable vm-import-controller: %v", err)
	}
	return nil
}

// listVMImports lists all VM imports present in Harvester
func listVMImports(ctx *cli.Context) error {

	c, err := GetHarvesterClient(ctx)

	if err != nil {
		return err
	}

	// An import lives in the same namespace as its source, which is not necessarily
	// harvester-system, so list across every namespace unless one was asked for.
	request := c.HarvesterhciV1beta1().RESTClient().Get().Resource("virtualmachineimports.migration")
	if namespace := ctx.String("namespace"); namespace != "" {
		request = request.Namespace(namespace)
	}

	vmImportResultRaw, err := request.DoRaw(context.Background())

	if err != nil {
		return fmt.Errorf("failed to list VM imports: %v", err)
	}

	var vmImportList VMImportV1.VirtualMachineImportList
	err = json.Unmarshal(vmImportResultRaw, &vmImportList)

	if err != nil {
		return fmt.Errorf("failed to unmarshal VM import list: %v", err)
	}

	writer := rcmd.NewTableWriter([][]string{
		{"NAME", "Name"},
		{"NAMESPACE", "Namespace"},
		{"VM NAME", "VMName"},
		{"STATUS", "Status"},
		{"SOURCE_CLUSTER", "SourceCluster"},
		{"CLUSTER_TYPE", "ClusterType"},
	}, ctxv1)

	defer func() { _ = writer.Close() }()

	for _, vmImport := range vmImportList.Items {
		writer.Write(&VMImportData{
			Name:          vmImport.Name,
			Namespace:     vmImport.Namespace,
			VMName:        vmImport.Spec.VirtualMachineName,
			Status:        string(vmImport.Status.Status),
			SourceCluster: vmImport.Spec.SourceCluster.Name,
			ClusterType:   vmImport.Spec.SourceCluster.Kind,
		})
	}

	return writer.Err()
}

func configureVMImport(ctx *cli.Context) error {

	if ctx.NArg() != 1 {
		return fmt.Errorf("VM import name is required, only 1 argument is allowed")
	}

	c, err := GetHarvesterClient(ctx)

	if err != nil {
		return err
	}

	sourceType := ctx.String("source-cluster-type")

	resourceType, _, err := resolveSourceClusterType(sourceType)
	if err != nil {
		return err
	}

	// Only the 'ova' source type can go without credentials, the other two have a non-optional
	// credentials field in their spec.
	var credentials *corev1.SecretReference
	if secret := ctx.String("credentials-secret"); secret != "" {
		credentialsNS, credentialsName, err := getNamespaceAndName(ctx, secret)
		if err != nil {
			return fmt.Errorf("failed to get namespace and name of the secret: %v", err)
		}
		credentials = &corev1.SecretReference{
			Name:      credentialsName,
			Namespace: credentialsNS,
		}
	} else if sourceType != "ova" {
		return fmt.Errorf("credentials-secret is required for %s source cluster type", sourceType)
	}

	// The API server rejects a body whose metadata.namespace differs from the namespace in the
	// request URL, so both have to come from --source-cluster-namespace.
	objectMeta := v1.ObjectMeta{
		Name:      ctx.Args().First(),
		Namespace: ctx.String("source-cluster-namespace"),
	}

	var createVMSourceBody []byte
	switch sourceType {
	case "vmware":
		if ctx.String("region") != "" {
			return fmt.Errorf("region is not supported for vmware source cluster type")
		}

		if ctx.String("dc") == "" {
			return fmt.Errorf("dc is required for vmware source cluster type")
		}
		vmWareSource := VMImportV1.VmwareSource{
			TypeMeta: v1.TypeMeta{
				Kind:       "VmwareSource",
				APIVersion: harvesterMigrationAPIGroup,
			},
			ObjectMeta: objectMeta,
			Spec: VMImportV1.VmwareSourceSpec{

				EndpointAddress: ctx.String("endpoint"),
				Datacenter:      ctx.String("dc"),
				Credentials:     *credentials,
			},
		}
		createVMSourceBody, err = json.Marshal(vmWareSource)
		if err != nil {
			return fmt.Errorf("failed to marshal vmware source: %v", err)
		}

	case "openstack":
		if ctx.String("dc") != "" {
			return fmt.Errorf("dc is not supported for openstack source cluster type")
		}

		if ctx.String("region") == "" {
			return fmt.Errorf("region is required for openstack source cluster type")
		}
		openStackSource := VMImportV1.OpenstackSource{
			TypeMeta: v1.TypeMeta{
				Kind:       "OpenstackSource",
				APIVersion: harvesterMigrationAPIGroup,
			},
			ObjectMeta: objectMeta,
			Spec: VMImportV1.OpenstackSourceSpec{
				EndpointAddress: ctx.String("endpoint"),
				Region:          ctx.String("region"),
				Credentials:     *credentials,
			},
		}
		createVMSourceBody, err = json.Marshal(openStackSource)

		if err != nil {
			return fmt.Errorf("failed to marshal openstack source: %v", err)
		}

	case "ova":
		if ctx.String("dc") != "" {
			return fmt.Errorf("dc is not supported for ova source cluster type")
		}

		if ctx.String("region") != "" {
			return fmt.Errorf("region is not supported for ova source cluster type")
		}

		httpTimeout := ctx.Int("http-timeout")
		ovaSource := VMImportV1.OvaSource{
			TypeMeta: v1.TypeMeta{
				Kind:       "OvaSource",
				APIVersion: harvesterMigrationAPIGroup,
			},
			ObjectMeta: objectMeta,
			Spec: VMImportV1.OvaSourceSpec{
				Url:              ctx.String("endpoint"),
				OvaSourceOptions: VMImportV1.OvaSourceOptions{HttpTimeoutSeconds: &httpTimeout},
				Credentials:      credentials,
			},
		}
		createVMSourceBody, err = json.Marshal(ovaSource)

		if err != nil {
			return fmt.Errorf("failed to marshal ova source: %v", err)
		}
	}

	_, err = c.HarvesterhciV1beta1().RESTClient().Post().Resource(resourceType + ".migration").Namespace(ctx.String("source-cluster-namespace")).Body(createVMSourceBody).DoRaw(context.Background())
	if err != nil {
		return fmt.Errorf("failed to create %s for VM import: %v", resourceType, err)
	}
	logrus.Infof("VM import source created successfully (%s)", ctx.Args().First())

	return nil
}

// deleteVMImportSource deletes a VM import source
func deleteVMImportSource(ctx *cli.Context) error {
	if ctx.NArg() != 1 {
		return fmt.Errorf("VM import name is required, only 1 argument is allowed")
	}

	c, err := GetHarvesterClient(ctx)

	if err != nil {
		return err
	}

	resourceType, _, err := resolveSourceClusterType(ctx.String("source-cluster-type"))
	if err != nil {
		return err
	}

	err = c.HarvesterhciV1beta1().RESTClient().Delete().Resource(resourceType + ".migration").Namespace(ctx.String("namespace")).Name(ctx.Args().First()).Do(context.Background()).Error()
	if err != nil {
		return fmt.Errorf("failed to delete %s for VM import: %v", resourceType, err)
	}
	logrus.Infof("VM import source deleted successfully (%s)", ctx.Args().First())

	return nil
}

func createVMImport(ctx *cli.Context) error {

	if ctx.NArg() != 1 {
		return fmt.Errorf("VM import name is required, only 1 argument is allowed")
	}

	c, err := GetHarvesterClient(ctx)

	if err != nil {
		return err
	}

	_, resourceKind, err := resolveSourceClusterType(ctx.String("source-cluster-type"))
	if err != nil {
		return err
	}

	var netMap []VMImportV1.NetworkMapping
	for _, mapping := range ctx.StringSlice("network-mapping") {
		mappingSplit := strings.Split(mapping, ":")
		if len(mappingSplit) != 2 {
			return fmt.Errorf("invalid mapping format: %v, must be <source-network>:<target-network>", mapping)
		}

		netMap = append(netMap, VMImportV1.NetworkMapping{
			SourceNetwork:      mappingSplit[0],
			DestinationNetwork: mappingSplit[1],
		})
	}

	vmImport := VMImportV1.VirtualMachineImport{
		TypeMeta: v1.TypeMeta{
			Kind:       "VirtualMachineImport",
			APIVersion: harvesterMigrationAPIGroup,
		},
		ObjectMeta: v1.ObjectMeta{
			Name: ctx.Args().First(),
			// Has to match the namespace the request is POSTed to, the API server rejects a mismatch.
			Namespace: ctx.String("source-cluster-namespace"),
		},
		Spec: VMImportV1.VirtualMachineImportSpec{
			SourceCluster: corev1.ObjectReference{
				Name:       ctx.String("source-cluster"),
				Kind:       resourceKind,
				Namespace:  ctx.String("source-cluster-namespace"),
				APIVersion: harvesterMigrationAPIGroup,
			},
			VirtualMachineName: ctx.String("vm-name"),
			Mapping:            netMap,
		},
	}

	createVMImportBody, err := json.Marshal(vmImport)
	if err != nil {
		return fmt.Errorf("failed to marshal vm import: %v", err)
	}

	_, err = c.HarvesterhciV1beta1().RESTClient().Post().Resource("virtualmachineimports.migration").Namespace(ctx.String("source-cluster-namespace")).Body(createVMImportBody).DoRaw(context.Background())
	if err != nil {
		return fmt.Errorf("failed to create vm import: %v", err)
	}

	logrus.Infof("VM import created successfully (%s)", ctx.Args().First())

	return nil
}

// deleteVMImport deletes a VM import
func deleteVMImport(ctx *cli.Context) error {

	if ctx.NArg() != 1 {
		return fmt.Errorf("VM import name is required, only 1 argument is allowed")
	}

	c, err := GetHarvesterClient(ctx)

	if err != nil {
		return err
	}

	err = c.HarvesterhciV1beta1().RESTClient().Delete().Resource("virtualmachineimports.migration").Namespace(ctx.String("namespace")).Name(ctx.Args().First()).Do(context.Background()).Error()
	if err != nil {
		return fmt.Errorf("failed to delete vm import: %v", err)
	}

	logrus.Infof("VM import deleted successfully (%s)", ctx.Args().First())

	return nil
}
