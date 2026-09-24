package cmd

import (
	"testing"

	"github.com/harvester/harvester/pkg/apis/harvesterhci.io/v1beta1"
	"gopkg.in/yaml.v3"
	"k8s.io/apimachinery/pkg/api/resource"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestHandleCPUOverCommittment(t *testing.T) {
	cpuNumber := int64(4)
	overCommitSettingMap := map[string]int{
		"cpu":    1600,
		"memory": 150,
		"disk":   200,
	}

	result := HandleCPUOverCommittment(overCommitSettingMap, cpuNumber)
	if result.MilliValue() != 250 {
		t.Errorf("Expected 250m, got %dm", result.MilliValue())
	}
}

func TestHandleMemoryOverCommittment(t *testing.T) {
	memoryLimit := "3G"
	overCommitSettingMap := map[string]int{
		"cpu":    1600,
		"memory": 150,
		"disk":   200,
	}

	result := HandleMemoryOverCommittment(overCommitSettingMap, memoryLimit)
	if result.ScaledValue(resource.Giga) != 2 {
		t.Errorf("Expected 2G, got %dM", result.ScaledValue(resource.Mega))
	}
}

func TestMergeOptionsInUserData(t *testing.T) {
	userData := `ssh_authorized_keys:
  - ssh-rsa AAAAB3NzaC1yc2EAAA ... custom@foo
packages:
  - docker
runcmd:
  - docker run -d --restart=unless-stopped -p 80:80 rancher/hello-world
`

	sshKey := &v1beta1.KeyPair{
		ObjectMeta: v1.ObjectMeta{
			Name: "test-ssh-key",
		},
		Spec: v1beta1.KeyPairSpec{
			PublicKey: "ssh-rsa AAAAB4MabD2zd3FBBB ... predef@bar",
		},
	}

	result, err := MergeOptionsInUserData(userData, defaultCloudInitUserData, sshKey)
	if err != nil {
		t.Errorf("Error merging options in user data: %v", err)
	}

	var resultMap map[string]interface{}
	err = yaml.Unmarshal([]byte(result), &resultMap)
	if err != nil {
		t.Errorf("Error unmarshalling result: %v", err)
	}

	sshAuthorizedKeys := resultMap["ssh_authorized_keys"].([]interface{})
	if len(sshAuthorizedKeys) != 2 {
		t.Errorf("Expected 2 ssh keys, got %d", len(sshAuthorizedKeys))
	}

	packages := resultMap["packages"].([]interface{})
	if len(packages) != 2 {
		t.Errorf("Expected 2 packages, got %d", len(packages))
	}

	runcmds := resultMap["runcmd"].([]interface{})
	if len(runcmds) != 4 {
		t.Errorf("Expected 4 runcmds, got %d", len(runcmds))
	}
}

// When no --user-data-* flag is given the default user data is passed on both sides of the merge,
// which used to duplicate every package and runcmd entry.
func TestMergeOptionsInUserDataIsIdempotent(t *testing.T) {
	result, err := MergeOptionsInUserData(defaultCloudInitUserData, defaultCloudInitUserData, nil)
	if err != nil {
		t.Fatalf("Error merging options in user data: %v", err)
	}

	var resultMap map[string]interface{}
	if err := yaml.Unmarshal([]byte(result), &resultMap); err != nil {
		t.Fatalf("Error unmarshalling result: %v", err)
	}

	if packages := resultMap["packages"].([]interface{}); len(packages) != 1 {
		t.Errorf("Expected 1 package, got %d: %v", len(packages), packages)
	}

	if runcmds := resultMap["runcmd"].([]interface{}); len(runcmds) != 3 {
		t.Errorf("Expected 3 runcmds, got %d: %v", len(runcmds), runcmds)
	}

	if _, ok := resultMap["ssh_authorized_keys"]; ok {
		t.Errorf("Expected no ssh_authorized_keys when no key is given, got %v", resultMap["ssh_authorized_keys"])
	}
}

// A keypair passed with --ssh-keyname has to end up in ssh_authorized_keys even when the user data
// does not already carry that section, otherwise the VM comes up without any way to log in.
func TestMergeOptionsInUserDataAddsKeyToDefaultUserData(t *testing.T) {
	sshKey := &v1beta1.KeyPair{
		ObjectMeta: v1.ObjectMeta{
			Name: "test-ssh-key",
		},
		Spec: v1beta1.KeyPairSpec{
			PublicKey: "ssh-rsa AAAAB4MabD2zd3FBBB ... predef@bar",
		},
	}

	result, err := MergeOptionsInUserData(defaultCloudInitUserData, defaultCloudInitUserData, sshKey)
	if err != nil {
		t.Fatalf("Error merging options in user data: %v", err)
	}

	var resultMap map[string]interface{}
	if err := yaml.Unmarshal([]byte(result), &resultMap); err != nil {
		t.Fatalf("Error unmarshalling result: %v", err)
	}

	keys, ok := resultMap["ssh_authorized_keys"].([]interface{})
	if !ok {
		t.Fatalf("Expected ssh_authorized_keys to be set, got %v", resultMap["ssh_authorized_keys"])
	}
	if len(keys) != 1 || keys[0] != sshKey.Spec.PublicKey {
		t.Errorf("Expected the public key to be the only entry, got %v", keys)
	}

	// Merging a second time must not add the key again.
	secondPass, err := MergeOptionsInUserData(result, defaultCloudInitUserData, sshKey)
	if err != nil {
		t.Fatalf("Error merging options in user data: %v", err)
	}
	var secondMap map[string]interface{}
	if err := yaml.Unmarshal([]byte(secondPass), &secondMap); err != nil {
		t.Fatalf("Error unmarshalling result: %v", err)
	}
	if keys := secondMap["ssh_authorized_keys"].([]interface{}); len(keys) != 1 {
		t.Errorf("Expected 1 ssh key after a second merge, got %d: %v", len(keys), keys)
	}
}
