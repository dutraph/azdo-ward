// Package config persists azdo-ward settings as YAML under the user's
// config dir ($XDG_CONFIG_HOME/azdo-ward/config.yaml on Linux,
// ~/Library/Application Support/azdo-ward/config.yaml on macOS).
//
// Multi-account is the default shape even when the user only has one org
// today, so adding a second later is just appending to the list. The
// legacy top-level fields (Organization, PAT) are kept mirrored to the
// active account for any call-site that reads them directly.
package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

const appName = "azdo-ward"

// Auth modes.
const (
	AuthPAT   = "pat"   // HTTP basic with a Personal Access Token
	AuthEntra = "entra" // Azure Entra ID bearer token via the `az` CLI
)

// Org is one Azure DevOps organization the tool can query.
type Org struct {
	Name string `yaml:"name"`
	// Auth selects how to authenticate: "pat" (default) or "entra".
	Auth string `yaml:"auth,omitempty"`
	// PAT is only used when Auth == "pat". For Entra there is no stored
	// secret — tokens are minted on demand from `az`.
	PAT string `yaml:"pat,omitempty"`
}

// Mode returns the effective auth mode, defaulting to PAT.
func (o *Org) Mode() string {
	if o.Auth == AuthEntra {
		return AuthEntra
	}
	return AuthPAT
}

// Config is the on-disk document.
type Config struct {
	Orgs   []Org  `yaml:"orgs"`
	Active string `yaml:"active"`

	// Legacy mirror of the active account — kept in sync on Save so older
	// readers keep working.
	Organization string `yaml:"organization,omitempty"`
	PAT          string `yaml:"pat,omitempty"`

	// Address the web server binds to. Empty means the built-in default.
	Listen string `yaml:"listen,omitempty"`
}

// Path returns the absolute config path, creating the parent dir.
func Path() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	d := filepath.Join(dir, appName)
	if err := os.MkdirAll(d, 0o700); err != nil {
		return "", err
	}
	return filepath.Join(d, "config.yaml"), nil
}

// Load reads the config, migrating a legacy single-account file into the
// multi-account list transparently. A missing file yields an empty,
// usable Config (not an error).
func Load() (*Config, error) {
	p, err := Path()
	if err != nil {
		return nil, err
	}
	b, err := os.ReadFile(p)
	if os.IsNotExist(err) {
		return &Config{}, nil
	}
	if err != nil {
		return nil, err
	}
	var c Config
	if err := yaml.Unmarshal(b, &c); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", p, err)
	}
	c.migrate()
	return &c, nil
}

// migrate folds a legacy top-level org/pat into the orgs list if the
// list is empty, so upgrading is seamless.
func (c *Config) migrate() {
	if len(c.Orgs) == 0 && c.Organization != "" {
		c.Orgs = append(c.Orgs, Org{Name: c.Organization, PAT: c.PAT, Auth: AuthPAT})
		c.Active = c.Organization
	}
	if c.Active == "" && len(c.Orgs) > 0 {
		c.Active = c.Orgs[0].Name
	}
}

// Save writes the config with 0600 perms, refreshing the legacy mirror.
func (c *Config) Save() error {
	if a := c.ActiveOrg(); a != nil {
		c.Organization = a.Name
		c.PAT = a.PAT
	}
	p, err := Path()
	if err != nil {
		return err
	}
	b, err := yaml.Marshal(c)
	if err != nil {
		return err
	}
	return os.WriteFile(p, b, 0o600)
}

// ActiveOrg returns the currently selected org, or nil if none.
func (c *Config) ActiveOrg() *Org {
	for i := range c.Orgs {
		if c.Orgs[i].Name == c.Active {
			return &c.Orgs[i]
		}
	}
	if len(c.Orgs) > 0 {
		return &c.Orgs[0]
	}
	return nil
}

// Upsert adds or updates an org by name and makes it active. auth is
// "pat" or "entra"; when empty it is left unchanged on an existing org and
// defaults to "pat" on a new one. For Entra orgs, pat is ignored.
func (c *Config) Upsert(name, pat, auth string) {
	for i := range c.Orgs {
		if c.Orgs[i].Name == name {
			if auth != "" {
				c.Orgs[i].Auth = auth
			}
			if auth == AuthEntra {
				c.Orgs[i].PAT = ""
			} else if pat != "" {
				c.Orgs[i].PAT = pat
			}
			c.Active = name
			return
		}
	}
	if auth == "" {
		auth = AuthPAT
	}
	o := Org{Name: name, Auth: auth}
	if auth == AuthPAT {
		o.PAT = pat
	}
	c.Orgs = append(c.Orgs, o)
	c.Active = name
}

// SetActive switches the active org. It returns false if the name is not
// in the list.
func (c *Config) SetActive(name string) bool {
	for i := range c.Orgs {
		if c.Orgs[i].Name == name {
			c.Active = name
			return true
		}
	}
	return false
}

// Remove deletes an org by name. If it was the active org, the first
// remaining org becomes active (or none). Returns false if not found.
func (c *Config) Remove(name string) bool {
	idx := -1
	for i := range c.Orgs {
		if c.Orgs[i].Name == name {
			idx = i
			break
		}
	}
	if idx < 0 {
		return false
	}
	c.Orgs = append(c.Orgs[:idx], c.Orgs[idx+1:]...)
	if c.Active == name {
		if len(c.Orgs) > 0 {
			c.Active = c.Orgs[0].Name
		} else {
			c.Active = ""
		}
	}
	return true
}
