package cmd

import (
	"bufio"
	"bytes"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"math/rand"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/docker/docker/pkg/namesgenerator"
	"github.com/harvester/harvester/pkg/apis/harvesterhci.io/v1beta1"
	harvclient "github.com/harvester/harvester/pkg/generated/clientset/versioned"
	"github.com/pkg/errors"
	"github.com/rancher/cli/cliclient"
	"github.com/rancher/cli/config"
	"github.com/rancher/norman/clientbase"
	ntypes "github.com/rancher/norman/types"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
	regen "github.com/zach-klippenstein/goregen"
	"gopkg.in/yaml.v3"
	"k8s.io/apimachinery/pkg/api/resource"
	kubeclient "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	VMv1 "kubevirt.io/api/core/v1"
)

const (
	letters = "abcdefghijklmnopqrstuvwxyz0123456789"
	cfgFile = "cli2.json"
)

var (
	// ManagementResourceTypes lists the types we use the management client for
	ManagementResourceTypes = []string{"cluster", "node", "project"}
	// ProjectResourceTypes lists the types we use the cluster client for
	ProjectResourceTypes = []string{"secret", "namespacedSecret", "workload"}
	// ClusterResourceTypes lists the types we use the project client for
	ClusterResourceTypes = []string{"persistentVolume", "storageClass", "namespace"}
)

type MemberData struct {
	Name       string
	MemberType string
	AccessType string
}

type RoleTemplate struct {
	ID          string
	Name        string
	Description string
}

type RoleTemplateBinding struct {
	ID      string
	User    string
	Role    string
	Created string
}

func loadAndVerifyCert(path string) (string, error) {
	caCert, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return verifyCert(caCert)
}

func verifyCert(caCert []byte) (string, error) {
	// replace the escaped version of the line break
	caCert = bytes.ReplaceAll(caCert, []byte(`\n`), []byte("\n"))

	block, _ := pem.Decode(caCert)

	if nil == block {
		return "", errors.New("No cert was found")
	}

	parsedCert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return "", err
	}

	if !parsedCert.IsCA {
		return "", errors.New("CACerts is not valid")
	}
	return string(caCert), nil
}

func loadConfig(ctx *cli.Context) (config.Config, error) {
	// path will always be set by the global flag default
	path := ctx.String("config")
	path = filepath.Join(path, cfgFile)

	cf := config.Config{
		Path:    path,
		Servers: make(map[string]*config.ServerConfig),
	}

	content, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return cf, nil
	} else if err != nil {
		return cf, err
	}

	err = json.Unmarshal(content, &cf)
	cf.Path = path

	return cf, err
}

func lookupConfig(ctx *cli.Context) (*config.ServerConfig, error) {
	cf, err := loadConfig(ctx)
	if err != nil {
		return nil, err
	}

	cs := cf.FocusedServer()
	if cs == nil {
		return nil, errors.New("no configuration found, run `login`")
	}

	return cs, nil
}

func GetClient(ctx *cli.Context) (*cliclient.MasterClient, error) {
	cf, err := lookupConfig(ctx)
	if err != nil {
		return nil, err
	}

	mc, err := cliclient.NewMasterClient(cf)
	if err != nil {
		return nil, err
	}

	return mc, nil
}

// GetResourceType maps an incoming resource type to a valid one from the schema
func GetResourceType(c *cliclient.MasterClient, resource string) (string, error) {
	if c.ManagementClient != nil {
		for key := range c.ManagementClient.Types {
			if strings.EqualFold(key, resource) {
				return key, nil
			}
		}
	}
	if c.ProjectClient != nil {
		for key := range c.ProjectClient.Types {
			if strings.EqualFold(key, resource) {
				return key, nil
			}
		}
	}
	if c.ClusterClient != nil {
		for key := range c.ClusterClient.Types {
			if strings.EqualFold(key, resource) {
				return key, nil
			}
		}
	}
	return "", fmt.Errorf("unknown resource type: %s", resource)
}

func Lookup(c *cliclient.MasterClient, name string, types ...string) (*ntypes.Resource, error) {
	var byName *ntypes.Resource

	for _, schemaType := range types {
		rt, err := GetResourceType(c, schemaType)
		if err != nil {
			logrus.Debugf("Error GetResourceType: %v", err)
			return nil, err
		}
		var schemaClient clientbase.APIBaseClientInterface
		// the schemaType dictates which client we need to use
		if c.ManagementClient != nil {
			if _, ok := c.ManagementClient.Types[rt]; ok {
				schemaClient = c.ManagementClient
			}
		}
		if c.ProjectClient != nil {
			if _, ok := c.ProjectClient.Types[rt]; ok {
				schemaClient = c.ProjectClient
			}
		}
		if c.ClusterClient != nil {
			if _, ok := c.ClusterClient.Types[rt]; ok {
				schemaClient = c.ClusterClient
			}
		}

		// Attempt to get the resource by ID
		var resource ntypes.Resource

		if err := schemaClient.ByID(schemaType, name, &resource); !clientbase.IsNotFound(err) && err != nil {
			logrus.Debugf("Error schemaClient.ByID: %v", err)
			return nil, err
		} else if err == nil && resource.ID == name {
			return &resource, nil
		}

		// Resource was not found assuming the ID, check if it's the name of a resource
		var collection ntypes.ResourceCollection

		listOpts := &ntypes.ListOpts{
			Filters: map[string]interface{}{
				"name":         name,
				"removed_null": 1,
			},
		}

		if err := schemaClient.List(schemaType, listOpts, &collection); !clientbase.IsNotFound(err) && err != nil {
			logrus.Debugf("Error schemaClient.List: %v", err)
			return nil, err
		}

		if len(collection.Data) > 1 {
			ids := []string{}
			for _, data := range collection.Data {
				ids = append(ids, data.ID)
			}
			return nil, fmt.Errorf("multiple resources of type %s found for name %s: %v", schemaType, name, ids)
		}

		// No matches for this schemaType, try the next one
		if len(collection.Data) == 0 {
			continue
		}

		if byName != nil {
			return nil, fmt.Errorf("multiple resources named %s: %s:%s, %s:%s", name, collection.Data[0].Type,
				collection.Data[0].ID, byName.Type, byName.ID)
		}

		byName = &collection.Data[0]

	}

	if byName == nil {
		return nil, fmt.Errorf("not found: %s", name)
	}

	return byName, nil
}

func RandomName() string {
	return strings.ReplaceAll(namesgenerator.GetRandomName(0), "_", "-")
}

// RejectFlagsAfterArgs fails a command when a flag was given after the first positional argument.
// Go's flag package stops parsing at the first non-flag token, so `vm create my-vm --dry-run`
// silently drops --dry-run and creates the VM for real. Refusing to run is a lot better than
// quietly doing the opposite of what was asked.
func RejectFlagsAfterArgs(ctx *cli.Context) error {
	for _, arg := range ctx.Args().Slice() {
		if len(arg) > 1 && strings.HasPrefix(arg, "-") {
			return fmt.Errorf("flag %q was given after a positional argument and would be ignored, put every flag before the argument", arg)
		}
	}
	return nil
}

// GuardFlagOrder attaches RejectFlagsAfterArgs to every leaf command of the tree, chaining it in
// front of any Before the command already has.
func GuardFlagOrder(commands []*cli.Command) {
	for _, command := range commands {
		if len(command.Subcommands) > 0 {
			GuardFlagOrder(command.Subcommands)
			continue
		}
		existingBefore := command.Before
		command.Before = func(ctx *cli.Context) error {
			if err := RejectFlagsAfterArgs(ctx); err != nil {
				return err
			}
			if existingBefore != nil {
				return existingBefore(ctx)
			}
			return nil
		}
	}
}

// RandomLetters returns a string with random letters of length n
func RandomLetters(n int) string {
	// The global source is seeded randomly since Go 1.20, rand.Seed is gone.
	b := make([]byte, n)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}

func appendTabDelim(buf *bytes.Buffer, value string) {
	if buf.Len() == 0 {
		buf.WriteString(value)
	} else {
		buf.WriteString("\t")
		buf.WriteString(value)
	}
}

func SimpleFormat(values [][]string) (string, string) {
	headerBuffer := bytes.Buffer{}
	valueBuffer := bytes.Buffer{}
	for _, v := range values {
		appendTabDelim(&headerBuffer, v[0])
		if strings.Contains(v[1], "{{") {
			appendTabDelim(&valueBuffer, v[1])
		} else {
			appendTabDelim(&valueBuffer, "{{."+v[1]+"}}")
		}
	}

	headerBuffer.WriteString("\n")
	valueBuffer.WriteString("\n")

	return headerBuffer.String(), valueBuffer.String()
}

func defaultAction(fn func(ctx *cli.Context) error) func(ctx *cli.Context) error {
	return func(ctx *cli.Context) error {
		if ctx.Bool("help") {
			err := cli.ShowAppHelp(ctx)
			if err != nil {
				logrus.Info("Issue encountered during executing help command")
			}
			return nil
		}
		return fn(ctx)
	}
}

// SplitOnColon splits an input string into an array of strings using column as a separator
func SplitOnColon(s string) []string {
	return strings.Split(s, ":")
}

// parseClusterAndProjectID comes from upstream Rancher CLI code and makes it possible to parse the cluster and project id from the tuples that come from the Rancher API
func parseClusterAndProjectID(id string) (string, string, error) {
	// Validate id
	// Examples:
	// c-qmpbm:p-mm62v
	// c-qmpbm:project-mm62v
	// See https://github.com/rancher/rancher/issues/14400
	if match, _ := regexp.MatchString("((local)|(c-[[:alnum:]]{5})):(p|project)-[[:alnum:]]{5}", id); match {
		parts := SplitOnColon(id)
		return parts[0], parts[1], nil
	}
	return "", "", fmt.Errorf("unable to extract clusterid and projectid from [%s]", id)
}

// getClusterNames maps cluster ID to name and defaults to ID if name is blank
func getClusterNames(ctx *cli.Context, c *cliclient.MasterClient) (map[string]string, error) {
	clusterNames := make(map[string]string)
	clusterCollection, err := c.ManagementClient.Cluster.List(defaultListOpts(ctx))
	if err != nil {
		return clusterNames, err
	}

	for _, cluster := range clusterCollection.Data {
		if cluster.Name == "" {
			clusterNames[cluster.ID] = cluster.ID
		} else {
			clusterNames[cluster.ID] = cluster.Name
		}
	}
	return clusterNames, nil
}

// baseListOptions comes from upstream Rancher CLI, it returns an empty ListOpts pointer
func baseListOpts() *ntypes.ListOpts {
	return &ntypes.ListOpts{
		Filters: map[string]interface{}{
			"limit": -1,
			"all":   true,
		},
	}
}

// defaultListOpts comes from upstream Rancher CLI code, it implements a way to handle lists of resources
func defaultListOpts(ctx *cli.Context) *ntypes.ListOpts {
	listOpts := baseListOpts()
	if ctx != nil && !ctx.Bool("all") {
		listOpts.Filters["removed_null"] = "1"
		listOpts.Filters["state_ne"] = []string{
			"inactive",
			"stopped",
			"removing",
		}
		delete(listOpts.Filters, "all")
	}
	if ctx != nil && ctx.Bool("system") {
		delete(listOpts.Filters, "system")
	} else {
		listOpts.Filters["system"] = "false"
	}
	return listOpts
}

// NewTrue returns a pointer to true
func NewTrue() *bool {
	b := true
	return &b
}

// runStrategyPtr returns a pointer to a VirtualMachineRunStrategy value, for use in VM specs.
func runStrategyPtr(s VMv1.VirtualMachineRunStrategy) *VMv1.VirtualMachineRunStrategy {
	return &s
}

// RandomID returns a random string used as an ID internally in Harvester.
func RandomID() string {
	res, err := regen.Generate("[a-z]{3}[0-9][a-z]")
	if err != nil {
		fmt.Println("Random function was not successful!")
		return ""
	}
	return res
}

// GetHarvesterClient creates a Client for Harvester from Config input
func GetHarvesterClient(ctx *cli.Context) (*harvclient.Clientset, error) {
	p := os.ExpandEnv(ctx.String("harvester-config"))

	clientConfig, err := clientcmd.BuildConfigFromFlags("", p)

	if err != nil {
		return &harvclient.Clientset{}, err
	}

	return harvclient.NewForConfig(clientConfig)

}

// GetKubeClient creates a Vanilla Kubernetes Client to query the Kubernetes-native API Objects
func GetKubeClient(ctx *cli.Context) (*kubeclient.Clientset, error) {
	p := os.ExpandEnv(ctx.String("harvester-config"))

	clientConfig, err := clientcmd.BuildConfigFromFlags("", p)

	if err != nil {
		return &kubeclient.Clientset{}, err
	}

	return kubeclient.NewForConfig(clientConfig)
}

// GetRESTClientAndConfig creates a *rest.Config pointer from a KUBECONFIG file
func GetRESTClientAndConfig(ctx *cli.Context) (clientConfig *rest.Config, err error) {
	p := os.ExpandEnv(ctx.String("harvester-config"))

	clientConfig, err = clientcmd.BuildConfigFromFlags("", p)

	if err != nil {
		err = fmt.Errorf("error during creation of Kube Config from File: %w", err)
		return
	}

	return
}

func GetRancherTokenMap(ctx *cli.Context) (tokenMap map[string]string, configMap map[string]*config.ServerConfig, err error) {
	rancherConfig, err := loadConfig(ctx)

	if err != nil {
		return
	}

	rancherServers := rancherConfig.Servers
	for _, ranchConfig := range rancherServers {
		serverURL, err := url.Parse(ranchConfig.URL)
		if err != nil {
			return tokenMap, configMap, err
		}
		tokenMap = make(map[string]string)
		tokenMap[serverURL.Host] = ranchConfig.TokenKey
		configMap = make(map[string]*config.ServerConfig)
		configMap[serverURL.Host] = ranchConfig

	}

	return
}

func GetSelectionFromInput(reader *bufio.Reader, tableSize int) (int, error) {

	errMessage := fmt.Sprintf("Invalid input, enter a number between 1 and %v: ", tableSize)
	var selection int

	for {
		input, err := reader.ReadString('\n')
		if err != nil {
			return 0, err
		}
		input = strings.TrimSpace(input)

		if input != "" {
			i, err := strconv.Atoi(input)
			if err != nil {
				fmt.Print(errMessage)
				continue
			}
			if i <= tableSize && i != 0 {
				selection = i
				break
			}
			fmt.Print(errMessage)
			continue
		}
	}
	return selection, nil
}

// HandleCPUOverCommittment calculates the CPU Request based on the CPU Limit and the CPU Overcommitment setting.
func HandleCPUOverCommittment(overCommitSettingMap map[string]int, cpuNumber int64) resource.Quantity {
	//cpuQuantiy := resource.NewQuantity(cpuNumber, resource.DecimalSI)
	cpuRequest := (1000 * cpuNumber) * 100 / int64(overCommitSettingMap["cpu"])
	return *resource.NewMilliQuantity(cpuRequest, resource.DecimalSI)
}

// HandleMemoryOverCommittment calculates the memory Request based on the memory Limit and the memory Overcommitment setting.
func HandleMemoryOverCommittment(overCommitSettingMap map[string]int, memory string) resource.Quantity {
	//cpuQuantiy := resource.NewQuantity(cpuNumber, resource.DecimalSI)
	memoryRequest := resource.MustParse(memory)
	memoryValue := memoryRequest.Value()

	return *resource.NewQuantity(memoryValue*100/int64(overCommitSettingMap["memory"]), resource.BinarySI)
}

// MergeOptionsInUserData merges the default user data and the provided public key with the user data provided by the user.
// The merge is idempotent: entries already present in the user data are never added a second time, so passing the
// default user data as the user data (which is what happens when no --user-data-* flag is given) is a no-op.
func MergeOptionsInUserData(userData string, defaultUserData string, sshKey *v1beta1.KeyPair) (string, error) {
	var err error
	var userDataMap map[string]interface{}
	var defaultUserDataMap map[string]interface{}

	err = yaml.Unmarshal([]byte(userData), &userDataMap)
	if err != nil {
		return "", err
	}

	err = yaml.Unmarshal([]byte(defaultUserData), &defaultUserDataMap)
	if err != nil {
		return "", err
	}

	if userDataMap == nil {
		userDataMap = map[string]interface{}{}
	}

	// The SSH key has to land in ssh_authorized_keys, otherwise cloud-init never installs it and the
	// VM is unreachable -- Harvester only records the key name in the harvesterhci.io/sshNames annotation.
	if sshKey != nil && sshKey.Spec.PublicKey != "" {
		userDataMap["ssh_authorized_keys"] = appendMissing(toSlice(userDataMap["ssh_authorized_keys"]), sshKey.Spec.PublicKey)
	}

	userDataMap["packages"] = appendMissing(toSlice(userDataMap["packages"]), toSlice(defaultUserDataMap["packages"])...)
	// The default runcmd entries install and start the guest agent, they have to run before anything the user asked for.
	userDataMap["runcmd"] = appendMissing(toSlice(defaultUserDataMap["runcmd"]), toSlice(userDataMap["runcmd"])...)

	for _, key := range []string{"ssh_authorized_keys", "packages", "runcmd"} {
		if len(toSlice(userDataMap[key])) == 0 {
			delete(userDataMap, key)
		}
	}

	mergedUserData, err := yaml.Marshal(userDataMap)

	if err != nil {
		return "", err
	}

	finalUserData := fmt.Sprintf("#cloud-config\n%s", string(mergedUserData))

	return finalUserData, nil

}

// toSlice normalises a cloud-init value that is expected to be a list into a []interface{}.
// A scalar is wrapped in a single-element slice, anything absent becomes nil.
func toSlice(value interface{}) []interface{} {
	switch v := value.(type) {
	case nil:
		return nil
	case []interface{}:
		return v
	default:
		return []interface{}{v}
	}
}

// appendMissing appends the given entries to list, skipping any that are already there.
// Entries are compared by their YAML rendering so that list-form runcmd entries compare correctly.
func appendMissing(list []interface{}, entries ...interface{}) []interface{} {
	seen := make(map[string]bool, len(list)+len(entries))
	key := func(entry interface{}) string {
		encoded, err := yaml.Marshal(entry)
		if err != nil {
			return fmt.Sprintf("%#v", entry)
		}
		return string(encoded)
	}

	for _, entry := range list {
		seen[key(entry)] = true
	}
	for _, entry := range entries {
		if k := key(entry); !seen[k] {
			seen[k] = true
			list = append(list, entry)
		}
	}
	return list
}

// toYAML marshals a Kubernetes object to YAML using its JSON field tags.
func toYAML(obj interface{}) (string, error) {
	j, err := json.Marshal(obj)
	if err != nil {
		return "", err
	}
	var m interface{}
	if err := json.Unmarshal(j, &m); err != nil {
		return "", err
	}
	b, err := yaml.Marshal(m)
	return string(b), err
}

func getNamespaceAndName(ctx *cli.Context, resourceName string) (resourceNS string, resource string, err error) {
	if strings.Contains(resourceName, "/") {
		parts := strings.Split(resourceName, "/")
		return parts[0], parts[1], nil
	}

	if ctx.String("namespace") != "" {
		return ctx.String("namespace"), resourceName, nil
	}

	return "", "", errors.New("could not determine namespace from resource name or context")
}
