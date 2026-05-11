module github.com/belgaied2/harvester-cli

go 1.25.7

replace (
	github.com/dgrijalva/jwt-go => github.com/dgrijalva/jwt-go v3.2.1-0.20200107013213-dc14462fd587+incompatible
	github.com/docker/distribution => github.com/docker/distribution v2.8.2+incompatible
	github.com/docker/docker => github.com/docker/docker v25.0.6+incompatible
	github.com/go-kit/kit => github.com/go-kit/kit v0.3.0

	// Pin gnostic-models to the version kube-openapi expects (uses gopkg.in/yaml.v3, not go.yaml.in/yaml/v3)
	github.com/google/gnostic-models => github.com/google/gnostic-models v0.6.9
	github.com/googleapis/gnostic => github.com/googleapis/gnostic v0.4.1
	github.com/knative/pkg => github.com/rancher/pkg v0.0.0-20190514055449-b30ab9de040e
	github.com/openshift/api => github.com/openshift/api v0.0.0-20191219222812-2987a591a72c
	github.com/openshift/client-go => github.com/openshift/client-go v0.0.0-20200521150516-05eb9880269c
	github.com/operator-framework/operator-lifecycle-manager => github.com/operator-framework/operator-lifecycle-manager v0.0.0-20190128024246-5eb7ae5bdb7a
	github.com/rancher/lasso => github.com/rancher/lasso v0.0.0-20241202185148-04649f379358
	github.com/rancher/rancher/pkg/apis => github.com/rancher/rancher/pkg/apis v0.0.0-20250828140533-07a90f09a491
	github.com/rancher/rancher/pkg/client => github.com/rancher/rancher/pkg/client v0.0.0-20250828140533-07a90f09a491

	helm.sh/helm/v3 => github.com/rancher/helm/v3 v3.15.1-rancher2
	k8s.io/api => k8s.io/api v0.33.7
	k8s.io/apiextensions-apiserver => k8s.io/apiextensions-apiserver v0.33.7
	k8s.io/apimachinery => k8s.io/apimachinery v0.33.7
	k8s.io/apiserver => k8s.io/apiserver v0.33.7
	k8s.io/cli-runtime => k8s.io/cli-runtime v0.33.7
	k8s.io/client-go => k8s.io/client-go v0.33.7
	k8s.io/cloud-provider => k8s.io/cloud-provider v0.33.7
	k8s.io/cluster-bootstrap => k8s.io/cluster-bootstrap v0.33.7
	k8s.io/code-generator => k8s.io/code-generator v0.33.7
	k8s.io/component-base => k8s.io/component-base v0.33.7
	k8s.io/component-helpers => k8s.io/component-helpers v0.33.7
	k8s.io/controller-manager => k8s.io/controller-manager v0.33.7
	k8s.io/cri-api => k8s.io/cri-api v0.33.7
	k8s.io/csi-translation-lib => k8s.io/csi-translation-lib v0.33.7
	k8s.io/dynamic-resource-allocation => k8s.io/dynamic-resource-allocation v0.33.7
	k8s.io/endpointslice => k8s.io/endpointslice v0.33.7
	k8s.io/gengo => k8s.io/gengo v0.0.0-20240826214909-a7b603a56eb7
	k8s.io/kms => k8s.io/kms v0.33.7
	k8s.io/kube-aggregator => k8s.io/kube-aggregator v0.33.7
	k8s.io/kube-controller-manager => k8s.io/kube-controller-manager v0.33.7
	k8s.io/kube-openapi => k8s.io/kube-openapi v0.0.0-20250318190949-c8a335a9a2ff
	k8s.io/kube-proxy => k8s.io/kube-proxy v0.33.7
	k8s.io/kube-scheduler => k8s.io/kube-scheduler v0.33.7
	k8s.io/kubectl => k8s.io/kubectl v0.33.7
	k8s.io/kubelet => k8s.io/kubelet v0.33.7
	k8s.io/kubernetes => k8s.io/kubernetes v1.33.7
	k8s.io/legacy-cloud-providers => k8s.io/legacy-cloud-providers v0.33.7
	k8s.io/metrics => k8s.io/metrics v0.33.7
	k8s.io/mount-utils => k8s.io/mount-utils v0.33.7
	k8s.io/pod-security-admission => k8s.io/pod-security-admission v0.33.7
	k8s.io/sample-apiserver => k8s.io/sample-apiserver v0.33.7
)

require (
	github.com/docker/docker v28.3.3+incompatible
	github.com/grantae/certinfo v0.0.0-20170412194111-59d56a35515b
	github.com/harvester/harvester v1.8.0
	github.com/harvester/vm-import-controller v1.8.0
	github.com/minio/pkg v1.1.14
	github.com/pkg/errors v0.9.1
	github.com/rancher/cli v1.0.0-alpha9.0.20210315153654-8de9f8e29aef
	github.com/rancher/norman v0.7.0
	github.com/rancher/types v0.0.0-20210123000350-7cb436b3f0b0
	github.com/sirupsen/logrus v1.9.3
	github.com/urfave/cli v1.22.17
	github.com/urfave/cli/v2 v2.25.1
	github.com/zach-klippenstein/goregen v0.0.0-20160303162051-795b5e3961ea
	gopkg.in/yaml.v3 v3.0.1
	k8s.io/api v0.35.0
	k8s.io/apimachinery v0.35.0
	k8s.io/client-go v12.0.0+incompatible
	k8s.io/kubectl v0.33.7
	k8s.io/metrics v0.34.1
	kubevirt.io/api v1.7.0
)

require (
	emperror.dev/errors v0.8.1 // indirect
	github.com/Azure/go-ansiterm v0.0.0-20230124172434-306776ec8161 // indirect
	github.com/MakeNowJust/heredoc v1.0.0 // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/blang/semver/v4 v4.0.0 // indirect
	github.com/c-bata/go-prompt v0.2.6 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/chai2010/gettext-go v1.0.2 // indirect
	github.com/cisco-open/operator-tools v0.37.0 // indirect
	github.com/cpuguy83/go-md2man/v2 v2.0.7 // indirect
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/emicklei/go-restful/v3 v3.12.2 // indirect
	github.com/evanphx/json-patch v5.9.11+incompatible // indirect
	github.com/evanphx/json-patch/v5 v5.9.11 // indirect
	github.com/exponent-io/jsonpath v0.0.0-20210407135951-1de76d718b3f // indirect
	github.com/fatih/camelcase v1.0.0 // indirect
	github.com/fsnotify/fsnotify v1.9.0 // indirect
	github.com/fxamacker/cbor/v2 v2.9.0 // indirect
	github.com/ghodss/yaml v1.0.0 // indirect
	github.com/go-errors/errors v1.4.2 // indirect
	github.com/go-logr/logr v1.4.3 // indirect
	github.com/go-openapi/jsonpointer v0.21.1 // indirect
	github.com/go-openapi/jsonreference v0.21.0 // indirect
	github.com/go-openapi/swag v0.23.1 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/google/btree v1.1.3 // indirect
	github.com/google/gnostic-models v0.7.0 // indirect
	github.com/google/go-cmp v0.7.0 // indirect
	github.com/google/gxui v0.0.0-20151028112939-f85e0a97b3a4 // indirect
	github.com/google/shlex v0.0.0-20191202100458-e7afc7fbc510 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/gorilla/websocket v1.5.4-0.20250319132907-e064f32e3674 // indirect
	github.com/gregjones/httpcache v0.0.0-20190611155906-901d90724c79 // indirect
	github.com/harvester/harvester-network-controller v1.6.0-rc3 // indirect
	github.com/hashicorp/go-version v1.7.0 // indirect
	github.com/iancoleman/orderedmap v0.3.0 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/josharian/intern v1.0.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/k8snetworkplumbingwg/network-attachment-definition-client v1.7.7 // indirect
	github.com/k8snetworkplumbingwg/whereabouts v0.9.3 // indirect
	github.com/kube-logging/logging-operator v0.0.0-20250424202944-7e1f9aad6e21 // indirect
	github.com/kube-logging/logging-operator/pkg/sdk v0.12.0 // indirect
	github.com/kubeovn/kube-ovn v1.14.10 // indirect
	github.com/kubereboot/kured v1.13.1 // indirect
	github.com/kubernetes-csi/external-snapshotter/client/v4 v4.2.0 // indirect
	github.com/liggitt/tabwriter v0.0.0-20181228230101-89fcab3d43de // indirect
	github.com/longhorn/longhorn-manager v1.10.0 // indirect
	github.com/mailru/easyjson v0.9.0 // indirect
	github.com/mattn/go-colorable v0.1.13 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/mattn/go-runewidth v0.0.16 // indirect
	github.com/mattn/go-tty v0.0.3 // indirect
	github.com/mitchellh/go-wordwrap v1.0.1 // indirect
	github.com/moby/spdystream v0.5.0 // indirect
	github.com/moby/term v0.5.0 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.3-0.20250322232337-35a7c28c31ee // indirect
	github.com/monochromegane/go-gitignore v0.0.0-20200626010858-205db1a8cc00 // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/mxk/go-flowrate v0.0.0-20140419014527-cca7078d478f // indirect
	github.com/openshift/custom-resource-status v1.1.2 // indirect
	github.com/ovn-org/libovsdb v0.7.0 // indirect
	github.com/peterbourgon/diskv v2.0.1+incompatible // indirect
	github.com/pkg/term v1.2.0-beta.2 // indirect
	github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring v0.82.0 // indirect
	github.com/prometheus/client_golang v1.23.2 // indirect
	github.com/prometheus/client_model v0.6.2 // indirect
	github.com/prometheus/common v0.66.1 // indirect
	github.com/prometheus/procfs v0.17.0 // indirect
	github.com/rancher/aks-operator v1.11.5 // indirect
	github.com/rancher/eks-operator v1.11.5 // indirect
	github.com/rancher/fleet/pkg/apis v0.12.3 // indirect
	github.com/rancher/gke-operator v1.11.5 // indirect
	github.com/rancher/lasso v0.2.7 // indirect
	github.com/rancher/rancher/pkg/apis v0.0.0 // indirect
	github.com/rancher/rke v1.8.5 // indirect
	github.com/rancher/system-upgrade-controller/pkg/apis v0.0.0-20250306000150-b1a9781accab // indirect
	github.com/rancher/wrangler v1.1.2 // indirect
	github.com/rancher/wrangler/v3 v3.2.4 // indirect
	github.com/rivo/uniseg v0.4.7 // indirect
	github.com/russross/blackfriday/v2 v2.1.0 // indirect
	github.com/smarty/assertions v1.16.0 // indirect
	github.com/spf13/cast v1.9.2 // indirect
	github.com/spf13/cobra v1.9.1 // indirect
	github.com/spf13/pflag v1.0.7 // indirect
	github.com/x448/float16 v0.8.4 // indirect
	github.com/xlab/treeprint v1.2.0 // indirect
	github.com/xrash/smetrics v0.0.0-20201216005158-039620a65673 // indirect
	go.opentelemetry.io/otel v1.37.0 // indirect
	go.opentelemetry.io/otel/trace v1.37.0 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	go.yaml.in/yaml/v2 v2.4.3 // indirect
	go.yaml.in/yaml/v3 v3.0.4 // indirect
	golang.org/x/crypto v0.48.0 // indirect
	golang.org/x/net v0.49.0 // indirect
	golang.org/x/oauth2 v0.31.0 // indirect
	golang.org/x/sync v0.19.0 // indirect
	golang.org/x/sys v0.41.0 // indirect
	golang.org/x/term v0.40.0 // indirect
	golang.org/x/text v0.34.0 // indirect
	golang.org/x/time v0.14.0 // indirect
	gomodules.xyz/jsonpatch/v2 v2.5.0 // indirect
	google.golang.org/protobuf v1.36.9 // indirect
	gopkg.in/evanphx/json-patch.v4 v4.12.0 // indirect
	gopkg.in/inf.v0 v0.9.1 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
	k8s.io/apiextensions-apiserver v0.34.1 // indirect
	k8s.io/apiserver v0.34.1 // indirect
	k8s.io/cli-runtime v0.34.1 // indirect
	k8s.io/component-base v0.34.1 // indirect
	k8s.io/component-helpers v0.33.7 // indirect
	k8s.io/klog/v2 v2.130.1 // indirect
	k8s.io/kube-openapi v0.32.8 // indirect
	k8s.io/kubernetes v1.34.1 // indirect
	k8s.io/utils v0.0.0-20251002143259-bc988d571ff4 // indirect
	kubevirt.io/containerized-data-importer-api v1.64.0 // indirect
	kubevirt.io/controller-lifecycle-operator-sdk/api v0.2.4 // indirect
	sigs.k8s.io/cluster-api v1.9.5 // indirect
	sigs.k8s.io/controller-runtime v0.21.0 // indirect
	sigs.k8s.io/json v0.0.0-20250730193827-2d320260d730 // indirect
	sigs.k8s.io/kustomize/api v0.19.0 // indirect
	sigs.k8s.io/kustomize/kyaml v0.19.0 // indirect
	sigs.k8s.io/randfill v1.0.0 // indirect
	sigs.k8s.io/structured-merge-diff/v4 v4.7.0 // indirect
	sigs.k8s.io/yaml v1.6.0 // indirect
)
