package config

import (
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/alphasoc/nfr/utils"
	yaml "gopkg.in/yaml.v2"
)

// DefaultLocation for config file.
const DefaultLocation = "/etc/nfr/config.yml"

// Config for nfr
type Config struct {
	// AlphaSOC server configuration
	Alphasoc struct {
		// AlphaSOC host server. Default: https://api.alpahsoc.net
		Host string `yaml:"host,omitempty"`
		// AlphaSOC api key. Required for start sending dns queries.
		APIKey string `yaml:"api_key,omitempty"`
	} `yaml:"alphasoc,omitempty"`

	// Network interface configuration.
	Network struct {
		// Interface on which nfr should listen. Default: (none)
		Interface string `yaml:"interface,omitempty"`
		// Protocols on which nfr should listen.
		// Possible values are udp and tcp.
		// Default: [udp]
		Protocols []string `yaml:"protocols,omitempty"`
		// Port on which nfr should listen. Default: 53
		Port int `yaml:"port,omitempty"`
	} `yaml:"network,omitempty"`

	// Log configuration.
	Log struct {
		// File to which nfr should log.
		// To print log to console use two special outputs: stderr or stdout
		// Default: stdout
		File string `yaml:"file,omitempty"`

		// Log level. Possibles values are: debug, info, warn, error
		// Default: info
		Level string `yaml:"level,omitempty"`
	} `yaml:"log,omitempty"`

	// Internal nfr data.
	Data struct {
		// File for internal data.
		// Default:
		// - linux /run/nfr.data
		// - win %AppData%/nfr.data
		File string `yaml:"file,omitempty"`
	} `yaml:"data,omitempty"`

	// Scope groups file.
	// The IP exclusion list is used to prune 'noisy' hosts, such as mail servers
	// or workstations within the IP ranges provided.
	// Finally, the domain scope is used to specify internal and trusted domains and
	// hostnames (supporting wildcards, e.g. *.google.com) to ignore.
	// If you do not scope domains, local DNS traffic will be forwarded to the AlphaSOC API for scoring.
	Scope struct {
		// File with scope groups . See ScopeConfig for more info.
		// Default: (none)
		File string `yaml:"file,omitempty"`
	} `yaml:"scope,omitempty"`

	// ScopeConfig is loaded when Scope.File is not empty or the default one is used:
	// groups:
	//   default:
	//     networks:
	//     - 10.0.0.0/8
	//     - 192.168.0.0/16
	//     - 172.16.0.0/12
	//     exclude:
	//       domains:
	//        - "*.arpa"
	//        - "*.lan"
	//        - "*.local"
	//        - "*.internal"
	ScopeConfig struct {
		Groups map[string]struct {
			// If packet source ip match this network, then the packet will be send to analyze.
			Networks []string `yaml:"networks,omitempty"`
			Exclude  struct {
				// Exclueds is list of network address excludes from monitoring networks.
				// This list has higher priority then networks list
				Networks []string `yaml:"networks,omitempty"`
				// Domains is list of fqdn. If dns packet fqdn match any
				// of this domains , then the packet will not be send to analyze.
				Domains []string `yaml:"domains,omitempty"`
			} `yaml:"exclude,omitempty"`
		} `yaml:"groups,omitempty"`
	} `yaml:"-"`

	// AlphaSOC events configuration.
	Events struct {
		// File where to store events. If not set then none events will be retrieved.
		// To print events to console use two special outputs: stderr or stdout
		// Default: "stderr"
		File string `yaml:"file,omitempty"`
		// Interval for polling events from AlphaSOC api. Default: 5m
		PollInterval time.Duration `yaml:"poll_interval,omitempty"`
	} `yaml:"events,omitempty"`

	// DNS queries configuration.
	Queries struct {
		// Buffer size for dns queries queue. If the size will be exceded then
		// nfr send quries to AlphaSOC api. Default: 65535
		BufferSize int `yaml:"buffer_size,omitempty"`
		// Interval for flushing queries to AlphaSOC api. Default: 30s
		FlushInterval time.Duration `yaml:"flush_interval,omitempty"`

		// Queries that were unable to send to AlphaSOC api.
		// If file is set, then unsent queries will be saved on disk
		// and then send again.
		// Pcap format is used to store queries. You can view it in
		// programs like tcpdump or whireshark.
		Failed struct {
			// File to store DNS Queries. Default: (none)
			File string `yaml:"file,omitempty"`
		} `yaml:"failed,omitempty"`
	} `yaml:"queries,omitempty"`
}

// New reads the config from file location. If file is not set
// then it tries to read from default location, if this fails, then
// default config is returned.
func New(file string) (*Config, error) {
	cfg := Config{}

	if file != "" {
		return Read(file)
	}
	if _, err := os.Stat(DefaultLocation); err == nil {
		return Read(DefaultLocation)
	}
	return cfg.setDefaults(), nil
}

// Read reads config from the given file.
func Read(file string) (*Config, error) {
	cfg := Config{}

	content, err := ioutil.ReadFile(file)
	if err != nil {
		return nil, fmt.Errorf("config: %s", err)
	}

	if err := yaml.Unmarshal(content, &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %s", err)
	}

	if err := cfg.loadScopeConfig(); err != nil {
		return nil, err
	}
	cfg.setDefaults()

	// some packages search protocols in slice. Guaratee the slice will be sorted.
	sort.Strings(cfg.Network.Protocols)

	return &cfg, cfg.validate()
}

// Save saves config to file.
func (cfg *Config) Save(file string) error {
	content, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}

	return ioutil.WriteFile(file, content, 0666)
}

// SaveDefault saves config to default file.
func (cfg *Config) SaveDefault() error {
	// create default directory if not exists.
	if err := os.MkdirAll(filepath.Dir(DefaultLocation), os.ModeDir); err != nil {
		return err
	}
	return cfg.Save(DefaultLocation)
}

func (cfg *Config) setDefaults() *Config {
	if cfg.Alphasoc.Host == "" {
		cfg.Alphasoc.Host = "https://api.alphasoc.net"
	}

	if len(cfg.Network.Protocols) == 0 {
		cfg.Network.Protocols = []string{"udp"}
	}

	if cfg.Network.Port == 0 {
		cfg.Network.Port = 53
	}

	if cfg.Events.File == "" {
		cfg.Events.File = "stderr"
	}

	if cfg.Log.File == "" {
		cfg.Log.File = "stdout"
	}
	if cfg.Log.Level == "" {
		cfg.Log.Level = "info"
	}

	if cfg.Data.File == "" {
		if runtime.GOOS == "windows" {
			cfg.Data.File = path.Join(os.Getenv("APPDATA"), "nfr.data")
		} else {
			cfg.Data.File = path.Join("/run", "nfr.data")
		}
	}

	if cfg.Events.PollInterval == 0 {
		cfg.Events.PollInterval = 5 * time.Minute
	}

	if cfg.Queries.BufferSize == 0 {
		cfg.Queries.BufferSize = 65535
	}
	if cfg.Queries.FlushInterval == 0 {
		cfg.Queries.FlushInterval = 30 * time.Second
	}

	cfg.loadScopeConfig()
	return cfg
}

func (cfg *Config) validate() error {
	if cfg.Network.Interface == "" {
		return fmt.Errorf("config: empty network.interface")
	}

	if _, err := net.InterfaceByName(cfg.Network.Interface); err != nil {
		return fmt.Errorf("config: can't open interface %s: %s", cfg.Network.Interface, err)
	}

	if len(cfg.Network.Protocols) == 0 {
		return fmt.Errorf("config: empty protocol list")
	}

	if len(cfg.Network.Protocols) > 2 {
		return fmt.Errorf("config: too many protocols in list (only tcp and udp are available)")
	}

	for _, proto := range cfg.Network.Protocols {
		if proto != "udp" && proto != "tcp" {
			return fmt.Errorf("config: invalid protocol %q name (only tcp and udp are available)", proto)
		}
	}

	if cfg.Network.Port < 0 || cfg.Network.Port > 65535 {
		return fmt.Errorf("config: invalid %d port number", cfg.Network.Port)
	}

	if err := validateFilename(cfg.Log.File, true); err != nil {
		return fmt.Errorf("config: %s", err)
	}
	if cfg.Log.Level != "debug" &&
		cfg.Log.Level != "info" &&
		cfg.Log.Level != "warn" &&
		cfg.Log.Level != "error" {
		return fmt.Errorf("config: invalid %s log level", cfg.Log.Level)
	}

	if err := validateFilename(cfg.Data.File, false); err != nil {
		return err
	}

	if cfg.Events.File != "" {
		if err := validateFilename(cfg.Events.File, true); err != nil {
			return fmt.Errorf("config: %s", err)
		}
	}

	if cfg.Events.PollInterval < 5*time.Second {
		return fmt.Errorf("config: events poll interval must be at least 5s")
	}

	if cfg.Queries.BufferSize < 64 {
		return fmt.Errorf("config: queries buffer size must be at least 64")
	}

	if cfg.Queries.FlushInterval < 5*time.Second {
		return fmt.Errorf("config: queries flush interval must be at least 5s")
	}

	if cfg.Queries.Failed.File != "" {
		if err := validateFilename(cfg.Queries.Failed.File, false); err != nil {
			return fmt.Errorf("config: %s", err)
		}
	}

	return nil
}

// validateFilename checks if file can be created.
func validateFilename(file string, noFileOutput bool) error {
	if noFileOutput && (file == "stdout" || file == "stderr") {
		return nil
	}

	dir := path.Dir(file)
	stat, err := os.Stat(dir)
	if err != nil {
		return fmt.Errorf("can't stat %s directory: %s", dir, err)
	}
	if !stat.IsDir() {
		return fmt.Errorf("%s is not directory", dir)
	}

	stat, err = os.Stat(file)
	if err == nil && !stat.Mode().IsRegular() {
		return fmt.Errorf("%s is not regular file", file)
	}
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("can't stat %s file: %s", file, err)
	}
	return nil
}

const defaultScope = `
groups:
  default:
    networks:
    - 10.0.0.0/8
    - 192.168.0.0/16
    - 172.16.0.0/12
    - fc00::/7
    exclude:
      domains:
       - "*.arpa"
       - "*.lan"
       - "*.local"
       - "*.internal"
      networks:
`

// load scope config from yaml file, or use default one.
func (cfg *Config) loadScopeConfig() (err error) {
	var content = []byte(defaultScope)
	if cfg.Scope.File != "" {
		content, err = ioutil.ReadFile(cfg.Scope.File)
		if err != nil {
			return fmt.Errorf("scope config: %s ", err)
		}
	}

	if err := yaml.Unmarshal(content, &cfg.ScopeConfig); err != nil {
		return fmt.Errorf("parse scope config: %s ", err)
	}

	return cfg.validateScopeConfig()
}

func (cfg *Config) validateScopeConfig() error {
	for _, group := range cfg.ScopeConfig.Groups {
		for _, n := range group.Networks {
			if _, _, err := net.ParseCIDR(n); err != nil {
				return fmt.Errorf("parse scope config: %s is not cidr", n)
			}
		}

		for _, n := range group.Exclude.Networks {
			_, _, err := net.ParseCIDR(n)
			ip := net.ParseIP(n)
			if err != nil && ip == nil {
				return fmt.Errorf("parse scope config: %s is not cidr nor ip", n)
			}
		}

		for _, domain := range group.Exclude.Domains {
			// TrimPrefix *. for multimatch domain
			if !utils.IsDomainName(domain) &&
				!utils.IsDomainName(strings.TrimPrefix(domain, "*.")) {
				return fmt.Errorf("parse scope config: %s is not valid domain name", domain)
			}
		}
	}
	return nil
}
