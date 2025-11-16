package diff

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/kitechsoftware/ldappy/internal/common/config"
	"github.com/stretchr/testify/assert"
)

func mockConfig(domain, org string) *config.Config {
	return &config.Config{
		LDAP: config.LDAP{
			Domain:       domain,
			Organization: org,
			BaseDN:       "dc=" + strings.ReplaceAll(domain, ".", ",dc="),
		},
	}
}

func TestDiff_HumanReadable(t *testing.T) {
	tests := []struct {
		name     string
		current  *config.Config
		desired  *config.Config
		expected string
	}{
		{
			name:     "no difference",
			current:  mockConfig("example.com", "ExampleOrg"),
			desired:  mockConfig("example.com", "ExampleOrg"),
			expected: "",
		},
		{
			name:     "single field changed",
			current:  mockConfig("example.com", "ExampleOrg"),
			desired:  mockConfig("ldap.example.com", "ExampleOrg"),
			expected: `+ domain = "ldap.example.com"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out := Diff(tt.current, tt.desired, HumanReadable)
			if tt.expected == "" {
				assert.Equal(t, "", out)
			} else {
				normalized := strings.ReplaceAll(out, "'", "\"") // <── NEW
				assert.Contains(t, normalized, tt.expected)
				assert.Contains(t, normalized, "--- state")
				assert.Contains(t, normalized, "+++ config")
			}
		})
	}
}

func TestDiff_JSONPatch(t *testing.T) {
	current := mockConfig("example.com", "ExampleOrg")
	desired := mockConfig("ldap.example.com", "ExampleOrg")

	out := Diff(current, desired, JSONPatch)
	var patches []Patch
	err := json.Unmarshal([]byte(out), &patches)
	assert.NoError(t, err)

	// Expect 2 changes: domain + base_dn
	assert.Len(t, patches, 2)

	// Find the domain patch
	var domainPatch, baseDNPatch *Patch
	for i := range patches {
		switch patches[i].Path {
		case "ldap.domain":
			domainPatch = &patches[i]
		case "ldap.base_dn":
			baseDNPatch = &patches[i]
		}
	}

	assert.NotNil(t, domainPatch)
	assert.Equal(t, "example.com", strings.Trim(domainPatch.OldValue, "'"))
	assert.Equal(t, "ldap.example.com", strings.Trim(domainPatch.NewValue, "'"))

	assert.NotNil(t, baseDNPatch)
	assert.Equal(t, "dc=example,dc=com", strings.Trim(baseDNPatch.OldValue, "'"))
	assert.Equal(t, "dc=ldap,dc=example,dc=com", strings.Trim(baseDNPatch.NewValue, "'"))
}

func TestDiff_JSONPatch_NoChange(t *testing.T) {
	current := mockConfig("example.com", "ExampleOrg")
	desired := mockConfig("example.com", "ExampleOrg")

	out := Diff(current, desired, JSONPatch)
	assert.Equal(t, "[]", strings.TrimSpace(out))
}

func TestDiffMode_String(t *testing.T) {
	assert.Equal(t, "human", HumanReadable.String())
	assert.Equal(t, "json", JSONPatch.String())
	assert.Equal(t, "unknown", DiffMode(99).String())
}
