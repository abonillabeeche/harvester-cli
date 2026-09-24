package cmd

import (
	"errors"
	"flag"
	"strings"
	"testing"

	"github.com/urfave/cli/v2"
)

// contextWithArgs builds the minimal cli.Context the usage guards look at.
func contextWithArgs(t *testing.T, args ...string) *cli.Context {
	t.Helper()
	set := flag.NewFlagSet("test", flag.ContinueOnError)
	if err := set.Parse(args); err != nil {
		t.Fatalf("parsing %v: %v", args, err)
	}
	return cli.NewContext(cli.NewApp(), set, nil)
}

func TestRejectFlagsAfterArgs(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr bool
	}{
		{name: "no arguments", args: nil},
		{name: "positional only", args: []string{"my-vm"}},
		{name: "several positionals", args: []string{"vm-a", "vm-b"}},
		{name: "a lone dash is not a flag", args: []string{"my-vm", "-"}},
		{name: "long flag after argument", args: []string{"my-vm", "--dry-run"}, wantErr: true},
		{name: "short flag after argument", args: []string{"my-vm", "-n"}, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := RejectFlagsAfterArgs(contextWithArgs(t, tt.args...))
			if tt.wantErr != (err != nil) {
				t.Fatalf("RejectFlagsAfterArgs(%v) = %v, wantErr %v", tt.args, err, tt.wantErr)
			}
		})
	}
}

func TestRejectUnknownSubcommands(t *testing.T) {
	parent := &cli.Command{
		Name: "image",
		Subcommands: cli.Commands{
			{Name: "list", Aliases: []string{"ls"}},
			{Name: "delete", Aliases: []string{"del", "rm"}},
		},
	}
	guard := RejectUnknownSubcommands(parent)

	tests := []struct {
		name    string
		args    []string
		wantErr bool
	}{
		{name: "no arguments falls through to the parent action", args: nil},
		{name: "known subcommand", args: []string{"list"}},
		{name: "known alias", args: []string{"rm", "an-image"}},
		{name: "help is appended by urfave after the guard is installed", args: []string{"help"}},
		{name: "unknown subcommand", args: []string{"bogus"}, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := guard(contextWithArgs(t, tt.args...))
			if tt.wantErr != (err != nil) {
				t.Fatalf("guard(%v) = %v, wantErr %v", tt.args, err, tt.wantErr)
			}
			if tt.wantErr && !strings.Contains(err.Error(), "expected one of: list, delete") {
				t.Errorf("error should list the valid subcommands, got %q", err)
			}
		})
	}
}

// GuardCommandUsage has to reach leaves nested under several parents, and must not drop a Before
// that the command already declared.
func TestGuardCommandUsageChainsExistingBefore(t *testing.T) {
	existingRan := false
	leaf := &cli.Command{
		Name:   "create",
		Before: func(*cli.Context) error { existingRan = true; return nil },
	}
	commands := []*cli.Command{{
		Name:        "image",
		Subcommands: cli.Commands{{Name: "catalog", Subcommands: cli.Commands{leaf}}},
	}}

	GuardCommandUsage(commands)

	if err := leaf.Before(contextWithArgs(t, "fedora/43")); err != nil {
		t.Fatalf("guarded Before rejected a valid invocation: %v", err)
	}
	if !existingRan {
		t.Error("the command's original Before was not called")
	}

	existingRan = false
	if err := leaf.Before(contextWithArgs(t, "fedora/43", "--dry-run")); err == nil {
		t.Error("a flag after a positional argument should be rejected")
	}
	if existingRan {
		t.Error("the original Before must not run once the guard has failed")
	}
}

func TestDefaultMarker(t *testing.T) {
	tests := []struct {
		name        string
		annotations map[string]string
		want        string
	}{
		{name: "not default", annotations: nil, want: ""},
		{name: "modern annotation", annotations: map[string]string{defaultStorageClassAnnotation: "true"}, want: "*"},
		{
			name:        "modern wins over beta",
			annotations: map[string]string{defaultStorageClassAnnotation: "true", betaDefaultStorageClassAnnotation: "false"},
			want:        "*",
		},
		{
			// The retail-store cluster looks exactly like this, and Harvester behaves as if it had
			// no default StorageClass at all.
			name:        "beta only is called out",
			annotations: map[string]string{defaultStorageClassAnnotation: "false", betaDefaultStorageClassAnnotation: "true"},
			want:        "(beta only, Harvester ignores it)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := defaultMarker(tt.annotations); got != tt.want {
				t.Errorf("defaultMarker(%v) = %q, want %q", tt.annotations, got, tt.want)
			}
		})
	}
}

func TestHintImportAddon(t *testing.T) {
	if err := hintImportAddon(nil); err != nil {
		t.Errorf("a nil error must stay nil, got %v", err)
	}

	other := errors.New("connection refused")
	if got := hintImportAddon(other); got != other {
		t.Errorf("an unrelated error must be passed through unchanged, got %v", got)
	}

	missing := errors.New("the server could not find the requested resource (get virtualmachineimports.migration.harvesterhci.io)")
	got := hintImportAddon(missing)
	if !strings.Contains(got.Error(), "harvester import enable") {
		t.Errorf("a missing CRD should point at the addon, got %v", got)
	}
	if !errors.Is(got, missing) {
		t.Error("the original error should stay wrapped")
	}
}

func TestValidateQuantityFlags(t *testing.T) {
	newContext := func(memory string) *cli.Context {
		set := flag.NewFlagSet("test", flag.ContinueOnError)
		set.String("memory", "", "")
		if err := set.Set("memory", memory); err != nil {
			t.Fatalf("setting memory=%q: %v", memory, err)
		}
		return cli.NewContext(cli.NewApp(), set, nil)
	}

	tests := []struct {
		memory  string
		wantErr bool
	}{
		{memory: "4Gi"},
		{memory: "512Mi"},
		{memory: "3G"},
		{memory: ""},                   // unset, the flag default applies
		{memory: "2", wantErr: true},   // bytes, the mutator would reject it
		{memory: "abc", wantErr: true}, // would panic in resource.MustParse
	}

	for _, tt := range tests {
		t.Run(tt.memory, func(t *testing.T) {
			err := validateQuantityFlags(newContext(tt.memory), "memory")
			if tt.wantErr != (err != nil) {
				t.Fatalf("validateQuantityFlags(%q) = %v, wantErr %v", tt.memory, err, tt.wantErr)
			}
		})
	}
}
