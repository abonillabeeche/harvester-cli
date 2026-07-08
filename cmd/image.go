package cmd

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/harvester/harvester/pkg/apis/harvesterhci.io/v1beta1"
	rcmd "github.com/rancher/cli/cmd"
	"github.com/rancher/cli/config"
	"github.com/sirupsen/logrus"
	cliv1 "github.com/urfave/cli"
	"github.com/urfave/cli/v2"
	k8smetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/clientcmd"
)

type ImageData struct {
	Name         string
	Id           string
	SourceType   string
	StorageClass string
	Url          string
}

type CatalogEntry struct {
	Id        int64  `json:"id,omitempty"`
	ShortName string `json:"shortName"`
	Version   string `json:"version"`
	Url       string `json:"url"`
	Build     string `json:"build"`
}

type Catalog struct {
	HarvesterImageCatalog map[string][]CatalogEntry `json:"HarvesterImageCatalog"`
}

type Os struct {
	Id             int64
	Name           string
	NumberOfImages string
}

const (
	defaultCatalogSource = "https://raw.githubusercontent.com/abonillabeeche/harvester-cli/main/image-metadata.json"
)

// TemplateCommand defines the CLI command that lists VM templates in Harvester
func ImageCommand() *cli.Command {
	return &cli.Command{
		Name:    "image",
		Aliases: []string{"img"},
		Usage:   "Manipulate VM images",
		Action:  imageList,
		Flags: []cli.Flag{
			&nsFlag,
		},
		Subcommands: cli.Commands{
			&cli.Command{
				Name:        "list",
				Aliases:     []string{"ls"},
				Usage:       "List VM images",
				Description: "\nLists all the VM images available in Harvester",
				ArgsUsage:   "",
				Action:      imageList,
				Flags: []cli.Flag{
					&nsFlag,
				},
			},
			&cli.Command{
				Name:        "create",
				Aliases:     []string{"add"},
				Usage:       "Creates a VM image",
				Description: "\nCreates a VM image from a source location",
				ArgsUsage:   "VM_IMAGE_DISPLAYNAME",
				Action:      imageCreate,
				Flags: []cli.Flag{
					&nsFlag,
					&cli.StringFlag{
						Name:     "source",
						Usage:    "Location from which the image will be put into Harvester, this should be either an HTTP(S) link or a path to a file that harvester will use to get the image",
						EnvVars:  []string{"HARVESTER_VM_IMAGE_LINK"},
						Required: true,
					},
					&cli.StringFlag{
						Name:     "description",
						Usage:    "Description of the VM Image",
						EnvVars:  []string{"HARVESTER_VM_IMAGE_DESCRIPTION"},
						Required: false,
					},
					&cli.StringFlag{
						Name:    "storage-class",
						Usage:   "StorageClass to use for the image (e.g. tworeplicas, harvester-longhorn)",
						EnvVars: []string{"HARVESTER_VM_IMAGE_SC"},
					},
					&cli.BoolFlag{
						Name:  "dry-run",
						Usage: "Print the YAML manifest that would be submitted without creating the resource",
					},
				},
			},
			&cli.Command{
				Name:        "catalog",
				Aliases:     []string{"cat"},
				Usage:       "lists an image catalog",
				Description: "\nShows a list of freely available linux images to download from URLs. If --namespace or --storage-class are not provided, the user is prompted interactively to pick from what exists on the cluster.",
				ArgsUsage:   "",
				Action:      imageCatalog,
				Flags: []cli.Flag{
					&nsFlag,
					&cli.StringFlag{
						Name:    "storage-class",
						Usage:   "StorageClass to use for the image (interactive prompt if omitted)",
						EnvVars: []string{"HARVESTER_VM_IMAGE_SC"},
					},
					&cli.StringFlag{
						Name:     "metadata-url",
						Usage:    "Location from which to get the metadata JSON file",
						EnvVars:  []string{"HARVESTER_CATALOG_METADATA"},
						Required: false,
						Value:    defaultCatalogSource,
					},
				},
				Subcommands: cli.Commands{
					&cli.Command{
						Name:        "init",
						Usage:       "Download the catalog metadata and cache it at ~/.harvester/image-metadata.json",
						Description: "\nDownloads the catalog JSON from --metadata-url (default: the public GitHub raw URL) and stores it alongside the harvester CLI config. Future 'catalog' commands prefer this local cache automatically, so subsequent runs work offline. Pass --force to overwrite an existing cache.",
						ArgsUsage:   "",
						Action:      imageCatalogInit,
						Flags: []cli.Flag{
							&cli.StringFlag{
								Name:    "metadata-url",
								Usage:   "URL to download the catalog metadata JSON from",
								EnvVars: []string{"HARVESTER_CATALOG_METADATA"},
								Value:   defaultCatalogSource,
							},
							&cli.BoolFlag{
								Name:  "force",
								Usage: "Overwrite an existing cached catalog",
							},
						},
					},
					&cli.Command{
						Name:        "list",
						Aliases:     []string{"ls"},
						Usage:       "List catalog images non-interactively (optionally filter by OS)",
						Description: "\nPrints the catalog entries without prompting. Pass an OS key (e.g. fedora) to filter.",
						ArgsUsage:   "[OS]",
						Action:      imageCatalogList,
						Flags: []cli.Flag{
							&cli.StringFlag{
								Name:    "metadata-url",
								Usage:   "Location from which to get the metadata JSON file",
								EnvVars: []string{"HARVESTER_CATALOG_METADATA"},
								Value:   defaultCatalogSource,
							},
						},
					},
					&cli.Command{
						Name:        "create",
						Aliases:     []string{"add"},
						Usage:       "Create a VM image from the catalog by <os>/<version> selector",
						Description: "\nCreates a VM image from a catalog entry (e.g. fedora/43, ubuntu/24.04). Uses the cluster's default StorageClass unless --storage-class is provided.",
						ArgsUsage:   "<OS>/<VERSION>",
						Action:      imageCatalogCreate,
						Flags: []cli.Flag{
							&nsFlag,
							&cli.StringFlag{
								Name:    "storage-class",
								Usage:   "StorageClass to use for the image (defaults to the cluster's default StorageClass)",
								EnvVars: []string{"HARVESTER_VM_IMAGE_SC"},
							},
							&cli.StringFlag{
								Name:  "display-name",
								Usage: "Display name for the image (defaults to the filename from the catalog URL)",
							},
							&cli.StringFlag{
								Name:    "description",
								Usage:   "Description of the VM Image",
								EnvVars: []string{"HARVESTER_VM_IMAGE_DESCRIPTION"},
							},
							&cli.BoolFlag{
								Name:  "dry-run",
								Usage: "Print the YAML manifest that would be submitted without creating the resource",
							},
							&cli.StringFlag{
								Name:    "metadata-url",
								Usage:   "Location from which to get the metadata JSON file",
								EnvVars: []string{"HARVESTER_CATALOG_METADATA"},
								Value:   defaultCatalogSource,
							},
						},
					},
				},
			},
		},
	}
}

var (
	ctxv1 = cliv1.NewContext(
		&cliv1.App{
			Name: "harvester",
		},
		flag.NewFlagSet("", flag.ContinueOnError),
		nil,
	)
)

func imageList(ctx *cli.Context) (err error) {
	c, err := GetHarvesterClient(ctx)

	if err != nil {
		return
	}

	imgList, err := c.HarvesterhciV1beta1().VirtualMachineImages(ctx.String("namespace")).List(context.TODO(), k8smetav1.ListOptions{})

	if err != nil {
		return
	}

	writer := rcmd.NewTableWriter([][]string{
		{"NAME", "Name"},
		{"ID", "Id"},
		{"SOURCE TYPE", "SourceType"},
		{"STORAGE CLASS", "StorageClass"},
		{"URL", "Url"},
	},
		ctxv1)

	defer writer.Close()

	for _, imgItem := range imgList.Items {

		writer.Write(&ImageData{
			Name:         imgItem.Spec.DisplayName,
			Id:           imgItem.Namespace + "/" + imgItem.Name,
			SourceType:   string(imgItem.Spec.SourceType),
			StorageClass: imgItem.Status.StorageClassName,
			Url:          imgItem.Spec.URL,
		})

	}

	return writer.Err()
}

// imageCreate create a VM Image in Harvester based on a URL and a display name as well as an optional description
func imageCreate(ctx *cli.Context) (err error) {
	if ctx.NArg() != 1 {
		err = fmt.Errorf("wrong number of arguments")
	}

	if err != nil {
		return
	}

	vmImageDisplayName := ctx.Args().Get(0)
	source := ctx.String("source")
	sourceType := "download"
	if !strings.HasPrefix(source, "http") {
		var fileInf fs.FileInfo
		if fileInf, err = os.Stat(source); err == nil {
			logrus.Debug("Source is a valid file!")
			filesize := fileInf.Size()
			sourceType = "upload"

			var rancherServerConfig *config.ServerConfig
			var harvesterURL string
			rancherServerConfig, harvesterURL, err = getHarvesterAPIFromConfig(ctx)

			if err != nil {
				return
			}
			logrus.Info("Successfully computed URL and credentials to Harvester!")

			var fileReader io.Reader
			fileReader, err = os.Open(source)
			if err != nil {
				return
			}

			var req *http.Request
			multipartBody := &bytes.Buffer{}
			writer := multipart.NewWriter(multipartBody)
			var part io.Writer
			part, err = writer.CreateFormFile("chunk", filepath.Base(source))

			if err != nil {
				return
			}

			_, err = io.Copy(part, fileReader)
			logrus.Info("Successfully preparated file for upload!")
			if err != nil {
				return
			}

			err = writer.Close()
			if err != nil {
				return
			}

			var vmImageCreateName string
			vmImageCreateName, err = createImageObjectInAPI(ctx, vmImageDisplayName, sourceType, source)
			if err != nil {
				return
			}
			logrus.Info("Image Object successfully created in Kubernetes API!")
			urlToSendFile := harvesterURL + "/v1/harvester/harvesterhci.io.virtualmachineimages/" + ctx.String("namespace") + "/" + vmImageCreateName + "?action=upload&size=" + strconv.FormatInt(filesize, 10)

			req, err = http.NewRequest("POST", urlToSendFile, multipartBody)
			if err != nil {
				return
			}
			var rootCAs *x509.CertPool
			rootCAs, err = x509.SystemCertPool()
			if err != nil {
				return err
			}
			pemBlock, _ := pem.Decode([]byte(rancherServerConfig.CACerts))
			var ownCert *x509.Certificate
			ownCert, err = x509.ParseCertificate(pemBlock.Bytes)
			if err != nil {
				return fmt.Errorf("invalid CA certification in Rancher configuration, %w", err)
			}

			rootCAs.AddCert(ownCert)

			req.Header.Add("Authorization", "Bearer "+rancherServerConfig.TokenKey)
			req.Header.Set("Content-Type", writer.FormDataContentType())
			tr := &http.Transport{
				TLSClientConfig: &tls.Config{
					RootCAs: rootCAs,
				},
			}
			httpClient := &http.Client{Transport: tr}

			logrus.Info("Uploading image file ...")
			var resp *http.Response
			resp, err = httpClient.Do(req)

			if err != nil {
				return err
			}

			if resp.Status == "200 OK" {
				logrus.Info("Successfully uploaded the image file! DONE!")
				return nil
			} else {
				return fmt.Errorf("uploading image file to harvester was not successful: %s", resp.Body)
			}

		} else {

			err = fmt.Errorf("source flag is neither a valid http link and nor a valid filepath")
			return

		}

	}
	imageID, err := createImageObjectInAPI(ctx, vmImageDisplayName, sourceType, source)
	if err == nil && imageID != "" {
		fmt.Printf("Image created: %s\n", imageID)
	}
	return

}

func createImageObjectInAPI(ctx *cli.Context, vmImageDisplayName string, sourceType string, source string) (vmImageCreateName string, err error) {

	if sourceType == "upload" {
		source = ""
	}

	vmImage := &v1beta1.VirtualMachineImage{
		TypeMeta: k8smetav1.TypeMeta{
			APIVersion: "harvesterhci.io/v1beta1",
			Kind:       "VirtualMachineImage",
		},
		ObjectMeta: k8smetav1.ObjectMeta{
			GenerateName: "image-",
			Namespace:    ctx.String("namespace"),
		},
		Spec: v1beta1.VirtualMachineImageSpec{
			Description:            ctx.String("description"),
			DisplayName:            vmImageDisplayName,
			SourceType:             v1beta1.VirtualMachineImageSourceType(sourceType),
			URL:                    source,
			TargetStorageClassName: ctx.String("storage-class"),
		},
	}

	if ctx.Bool("dry-run") {
		var out string
		out, err = toYAML(vmImage)
		if err != nil {
			err = fmt.Errorf("dry-run: %w", err)
			return
		}
		fmt.Printf("---\n%s", out)
		return
	}

	c, err := GetHarvesterClient(ctx)

	if err != nil {
		return
	}

	vmImageCreated, err := c.HarvesterhciV1beta1().VirtualMachineImages(ctx.String("namespace")).Create(context.TODO(), vmImage, k8smetav1.CreateOptions{})

	if err != nil {
		return
	}

	vmImageCreateName = vmImageCreated.Name
	return
}

func getHarvesterAPIFromConfig(ctx *cli.Context) (serverConfig *config.ServerConfig, harvesterKubeAPIServerURL string, err error) {

	p := os.ExpandEnv(ctx.String("harvester-config"))
	restConfig, err := clientcmd.BuildConfigFromFlags("", p)

	if err != nil {
		return
	}

	harvesterKubeAPIServerURL = restConfig.Host
	u, err := url.Parse(harvesterKubeAPIServerURL)

	if err != nil {
		return
	}

	harvesterKubeAPIServerHost := u.Host

	tokenMap, configMap, err := GetRancherTokenMap(ctx)

	if err != nil {
		return
	}

	var ok bool

	if _, ok = tokenMap[harvesterKubeAPIServerHost]; ok {
		serverConfig = configMap[harvesterKubeAPIServerHost]
		return
	} else {
		return nil, "", fmt.Errorf("not able to determine harvester API URL")
	}

}

// embeddedCatalog holds the image-metadata.json bundled with the binary. Populated by
// main via SetEmbeddedCatalog so //go:embed can live in the root package.
var embeddedCatalog []byte

// SetEmbeddedCatalog injects the bundled image-metadata.json bytes from the main package.
func SetEmbeddedCatalog(data []byte) {
	embeddedCatalog = data
}

// localCatalogCachePath returns the on-disk path where `catalog init` stashes the metadata
// JSON, or "" if the user's home directory can't be resolved.
func localCatalogCachePath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".harvester", "image-metadata.json")
}

// resolveCatalogSource picks which source to load the catalog from. Priority:
//  1. --metadata-url flag or HARVESTER_CATALOG_METADATA env var explicitly set
//  2. local cache at ~/.harvester/image-metadata.json if it exists
//  3. the default remote URL (will fall back to the embedded bundle on HTTP failure)
func resolveCatalogSource(ctx *cli.Context) string {
	if ctx.IsSet("metadata-url") {
		return ctx.String("metadata-url")
	}
	if p := localCatalogCachePath(); p != "" {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return defaultCatalogSource
}

// loadCatalog reads and parses catalog JSON from an HTTP(S) URL, file:// URL, or plain
// filesystem path. If the source is HTTP(S) and the fetch fails, it falls back to the
// binary's embedded catalog (with a warning) rather than erroring out — this is what
// makes offline use possible without any manual setup.
func loadCatalog(source string) (*Catalog, error) {
	var body []byte
	var err error

	switch {
	case strings.HasPrefix(source, "http://") || strings.HasPrefix(source, "https://"):
		body, err = httpFetch(source)
		if err != nil {
			if len(embeddedCatalog) == 0 {
				return nil, err
			}
			logrus.Warnf("fetching catalog from %s failed (%v); using catalog bundled with the CLI", source, err)
			body = embeddedCatalog
		}
	default:
		path := strings.TrimPrefix(source, "file://")
		logrus.Debugf("loading catalog from local file: %s", path)
		body, err = os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("reading catalog file %s: %w", path, err)
		}
	}

	var catalog Catalog
	if err := json.Unmarshal(body, &catalog); err != nil {
		return nil, fmt.Errorf("parsing catalog JSON: %w", err)
	}
	return &catalog, nil
}

// httpFetch downloads bytes from an HTTP(S) URL, treating any non-200 status as an error.
func httpFetch(source string) ([]byte, error) {
	logrus.Debugf("fetching catalog over HTTP: %s", source)
	resp, err := http.Get(source)
	if err != nil {
		return nil, fmt.Errorf("fetching catalog metadata: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetching catalog metadata: HTTP %s", resp.Status)
	}
	return io.ReadAll(resp.Body)
}

// catalogOSKeys returns the OS keys of the catalog in stable (sorted) order.
func catalogOSKeys(c *Catalog) []string {
	keys := make([]string, 0, len(c.HarvesterImageCatalog))
	for k := range c.HarvesterImageCatalog {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// filenameFromURL returns the last path segment of a URL, suitable as a display name.
func filenameFromURL(rawURL string) (string, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "", fmt.Errorf("invalid URL %q: %w", rawURL, err)
	}
	parts := strings.Split(u.EscapedPath(), "/")
	return parts[len(parts)-1], nil
}

func imageCatalog(ctx *cli.Context) (err error) {

	source := resolveCatalogSource(ctx)
	fmt.Printf("Image catalog source: %s\n\n", source)

	catalog, err := loadCatalog(source)
	if err != nil {
		return
	}

	writer := rcmd.NewTableWriter([][]string{
		{"NUMBER", "Id"},
		{"NAME", "Name"},
		{"NUMBER OF IMAGES", "NumberOfImages"},
	},
		ctxv1)

	osChoiceMap := make(map[int64]string)
	var i int64 = 0

	for os, imageList := range catalog.HarvesterImageCatalog {
		i++
		number := int64(len(imageList))
		writer.Write(&Os{
			Id:             i,
			Name:           os,
			NumberOfImages: strconv.FormatInt(number, 10),
		})
		osChoiceMap[i] = os

	}

	writer.Close()

	fmt.Println("Insert a number to select the image OS: ")
	reader := bufio.NewReader(os.Stdin)
	selection, err := GetSelectionFromInput(reader, len(osChoiceMap))
	if err != nil {
		return err
	}

	osSelection := osChoiceMap[int64(selection)]

	fmt.Printf("\nHere are the images available for %s\n\n", osSelection)

	writer = rcmd.NewTableWriter([][]string{
		{"NUMBER", "Id"},
		{"NAME", "ShortName"},
		{"VERSION", "Version"},
		{"BUILD", "Build"},
		{"URL", "Url"},
	}, ctxv1)

	imageChoiceMap := make(map[int64]string)

	for i, catalogItem := range catalog.HarvesterImageCatalog[osSelection] {
		catalogItem.Id = int64(i) + 1
		writer.Write(catalogItem)
		imageChoiceMap[catalogItem.Id] = catalogItem.Url
	}

	writer.Close()

	fmt.Printf("\nInsert a number to select an image to download: \n")
	selection, err = GetSelectionFromInput(reader, len(imageChoiceMap))
	if err != nil {
		return err
	}

	imageUrl := imageChoiceMap[int64(selection)]
	fmt.Printf("\nYour image URL is : %s\n", imageUrl)

	imageFilename, err := filenameFromURL(imageUrl)
	if err != nil {
		return err
	}

	if !ctx.IsSet("namespace") {
		chosenNs, err := promptForNamespaceSelection(ctx, reader)
		if err != nil {
			return err
		}
		if err := ctx.Set("namespace", chosenNs); err != nil {
			return fmt.Errorf("setting namespace flag: %w", err)
		}
	}

	if !ctx.IsSet("storage-class") {
		chosenSC, err := promptForStorageClassSelection(ctx, reader)
		if err != nil {
			return err
		}
		if err := ctx.Set("storage-class", chosenSC); err != nil {
			return fmt.Errorf("setting storage-class flag: %w", err)
		}
	}

	sc := ctx.String("storage-class")
	if sc == "" {
		logrus.Infof("Creating image %q in namespace %q using cluster default StorageClass", imageFilename, ctx.String("namespace"))
	} else {
		logrus.Infof("Creating image %q in namespace %q with StorageClass %q", imageFilename, ctx.String("namespace"), sc)
	}

	imageCreatedName, err := createImageObjectInAPI(ctx, imageFilename, "download", imageUrl)
	if err != nil {
		return fmt.Errorf("error during creation of image in Harvester %w", err)
	}

	logrus.Infof("Image was created in Harvester with display name %s and id %s", imageFilename, imageCreatedName)

	return nil
}

// promptForText reads a line and returns it trimmed. If empty, returns defaultValue.
// The prompt label already shown by the caller should include the "[default]" hint.
func promptForText(reader *bufio.Reader, defaultValue string) (string, error) {
	input, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	input = strings.TrimSpace(input)
	if input == "" {
		return defaultValue, nil
	}
	return input, nil
}

// promptForNamespaceSelection tries to list namespaces from the cluster and show a picker.
// If the caller lacks cluster-wide list permission (common with Rancher-proxied Harvester
// kubeconfigs), it falls back to a free-form text prompt with the current --namespace value
// as the default.
func promptForNamespaceSelection(ctx *cli.Context, reader *bufio.Reader) (string, error) {
	defaultNs := ctx.String("namespace")
	if defaultNs == "" {
		defaultNs = "default"
	}

	kube, err := GetKubeClient(ctx)
	if err != nil {
		logrus.Warnf("could not build kube client (%v); asking for namespace name instead", err)
		fmt.Printf("\nEnter the namespace to create the image in [%s]: ", defaultNs)
		return promptForText(reader, defaultNs)
	}
	nsList, err := kube.CoreV1().Namespaces().List(context.TODO(), k8smetav1.ListOptions{})
	if err != nil {
		logrus.Warnf("could not list namespaces (%v); asking for namespace name instead", err)
		fmt.Printf("\nEnter the namespace to create the image in [%s]: ", defaultNs)
		return promptForText(reader, defaultNs)
	}
	if len(nsList.Items) == 0 {
		logrus.Warn("no namespaces returned; asking for namespace name instead")
		fmt.Printf("\nEnter the namespace to create the image in [%s]: ", defaultNs)
		return promptForText(reader, defaultNs)
	}

	type nsRow struct {
		Id   int64
		Name string
	}
	writer := rcmd.NewTableWriter([][]string{
		{"NUMBER", "Id"},
		{"NAME", "Name"},
	}, ctxv1)
	nsChoiceMap := make(map[int64]string)
	var i int64
	for _, n := range nsList.Items {
		i++
		writer.Write(&nsRow{Id: i, Name: n.Name})
		nsChoiceMap[i] = n.Name
	}
	writer.Close()

	fmt.Println("\nInsert a number to select the namespace: ")
	sel, err := GetSelectionFromInput(reader, len(nsChoiceMap))
	if err != nil {
		return "", err
	}
	return nsChoiceMap[int64(sel)], nil
}

// promptForStorageClassSelection tries to list StorageClasses and show a picker. Falls
// back to a text prompt (empty input = cluster default) when listing fails.
func promptForStorageClassSelection(ctx *cli.Context, reader *bufio.Reader) (string, error) {
	kube, err := GetKubeClient(ctx)
	if err != nil {
		logrus.Warnf("could not build kube client (%v); asking for StorageClass name instead", err)
		fmt.Print("\nEnter the StorageClass name (empty = cluster default): ")
		return promptForText(reader, "")
	}
	scList, err := kube.StorageV1().StorageClasses().List(context.TODO(), k8smetav1.ListOptions{})
	if err != nil {
		logrus.Warnf("could not list StorageClasses (%v); asking for StorageClass name instead", err)
		fmt.Print("\nEnter the StorageClass name (empty = cluster default): ")
		return promptForText(reader, "")
	}
	if len(scList.Items) == 0 {
		logrus.Warn("no StorageClasses found on cluster; using cluster default (empty)")
		return "", nil
	}

	type scRow struct {
		Id      int64
		Name    string
		Default string
	}
	writer := rcmd.NewTableWriter([][]string{
		{"NUMBER", "Id"},
		{"NAME", "Name"},
		{"DEFAULT", "Default"},
	}, ctxv1)
	scChoiceMap := make(map[int64]string)
	var i int64 = 1
	writer.Write(&scRow{Id: i, Name: "(use cluster default)", Default: ""})
	scChoiceMap[i] = ""
	for _, sc := range scList.Items {
		i++
		def := ""
		if sc.Annotations["storageclass.kubernetes.io/is-default-class"] == "true" {
			def = "*"
		}
		writer.Write(&scRow{Id: i, Name: sc.Name, Default: def})
		scChoiceMap[i] = sc.Name
	}
	writer.Close()

	fmt.Println("\nInsert a number to select the StorageClass (* = cluster default): ")
	sel, err := GetSelectionFromInput(reader, len(scChoiceMap))
	if err != nil {
		return "", err
	}
	return scChoiceMap[int64(sel)], nil
}

// imageCatalogList prints catalog entries non-interactively. Optional first arg filters by OS key.
func imageCatalogList(ctx *cli.Context) error {
	catalog, err := loadCatalog(resolveCatalogSource(ctx))
	if err != nil {
		return err
	}

	if ctx.NArg() > 0 {
		osKey := ctx.Args().First()
		entries, ok := catalog.HarvesterImageCatalog[osKey]
		if !ok {
			return fmt.Errorf("unknown OS %q. Available: %s", osKey, strings.Join(catalogOSKeys(catalog), ", "))
		}
		writer := rcmd.NewTableWriter([][]string{
			{"VERSION", "Version"},
			{"BUILD", "Build"},
			{"SHORT NAME", "ShortName"},
			{"URL", "Url"},
		}, ctxv1)
		defer writer.Close()
		for _, e := range entries {
			entry := e
			writer.Write(&entry)
		}
		return writer.Err()
	}

	type catalogRow struct {
		OS        string
		Version   string
		Build     string
		ShortName string
		Url       string
	}
	writer := rcmd.NewTableWriter([][]string{
		{"OS", "OS"},
		{"VERSION", "Version"},
		{"BUILD", "Build"},
		{"SHORT NAME", "ShortName"},
		{"URL", "Url"},
	}, ctxv1)
	defer writer.Close()

	for _, osKey := range catalogOSKeys(catalog) {
		for _, e := range catalog.HarvesterImageCatalog[osKey] {
			writer.Write(&catalogRow{
				OS:        osKey,
				Version:   e.Version,
				Build:     e.Build,
				ShortName: e.ShortName,
				Url:       e.Url,
			})
		}
	}
	return writer.Err()
}

// imageCatalogCreate creates a VM image from a catalog entry selected by <os>/<version>.
// If multiple entries share the same version (e.g. multiple Rocky Linux 9.5 builds), the last
// match wins — catalog arrays are ordered oldest→newest.
func imageCatalogCreate(ctx *cli.Context) error {
	if ctx.NArg() != 1 {
		return fmt.Errorf("expected exactly one argument in the form <os>/<version> (e.g. fedora/43)")
	}
	selector := ctx.Args().First()
	parts := strings.SplitN(selector, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return fmt.Errorf("selector must be in the form <os>/<version> (got %q)", selector)
	}
	osKey, version := parts[0], parts[1]

	catalog, err := loadCatalog(resolveCatalogSource(ctx))
	if err != nil {
		return err
	}

	entries, ok := catalog.HarvesterImageCatalog[osKey]
	if !ok {
		return fmt.Errorf("unknown OS %q. Available: %s", osKey, strings.Join(catalogOSKeys(catalog), ", "))
	}

	var matches []CatalogEntry
	for _, e := range entries {
		if e.Version == version {
			matches = append(matches, e)
		}
	}
	if len(matches) == 0 {
		seen := map[string]struct{}{}
		var versions []string
		for _, e := range entries {
			if _, ok := seen[e.Version]; ok {
				continue
			}
			seen[e.Version] = struct{}{}
			versions = append(versions, e.Version)
		}
		return fmt.Errorf("no image with version %q for %s. Available versions: %s", version, osKey, strings.Join(versions, ", "))
	}
	chosen := matches[len(matches)-1]
	if len(matches) > 1 {
		logrus.Infof("Multiple builds (%d) for %s/%s; using build %q", len(matches), osKey, version, chosen.Build)
	}

	logrus.Infof("Selected: %s (build %s)", chosen.ShortName, chosen.Build)
	logrus.Infof("URL: %s", chosen.Url)

	displayName := ctx.String("display-name")
	if displayName == "" {
		displayName, err = filenameFromURL(chosen.Url)
		if err != nil {
			return err
		}
	}

	if sc := ctx.String("storage-class"); sc == "" {
		logrus.Info("Using cluster default StorageClass")
	} else {
		logrus.Infof("Using StorageClass: %s", sc)
	}
	logrus.Infof("Namespace: %s", ctx.String("namespace"))

	imageID, err := createImageObjectInAPI(ctx, displayName, "download", chosen.Url)
	if err != nil {
		return fmt.Errorf("creating image in Harvester: %w", err)
	}

	if ctx.Bool("dry-run") {
		return nil
	}

	logrus.Infof("Image was created in Harvester with display name %s and id %s", displayName, imageID)
	return nil
}

// imageCatalogInit downloads the catalog JSON and writes it to ~/.harvester/image-metadata.json
// so future 'catalog' commands can work offline. --force overwrites an existing cache.
func imageCatalogInit(ctx *cli.Context) error {
	source := ctx.String("metadata-url")

	dest := localCatalogCachePath()
	if dest == "" {
		return fmt.Errorf("could not resolve home directory to place the catalog cache")
	}

	if _, err := os.Stat(dest); err == nil && !ctx.Bool("force") {
		return fmt.Errorf("%s already exists; re-run with --force to overwrite", dest)
	}

	fmt.Printf("Downloading catalog from: %s\n", source)
	fmt.Printf("Saving to:                %s\n\n", dest)

	body, err := httpFetch(source)
	if err != nil {
		return err
	}

	var catalog Catalog
	if err := json.Unmarshal(body, &catalog); err != nil {
		return fmt.Errorf("downloaded content is not valid catalog JSON: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
		return fmt.Errorf("creating %s: %w", filepath.Dir(dest), err)
	}
	if err := os.WriteFile(dest, body, 0644); err != nil {
		return fmt.Errorf("writing %s: %w", dest, err)
	}

	fmt.Printf("Cached catalog with %d OS group(s). Future 'image catalog' runs will use this file automatically.\n", len(catalog.HarvesterImageCatalog))
	fmt.Println("Re-run with --force to refresh.")
	return nil
}
