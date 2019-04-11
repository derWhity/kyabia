package internal

import (
	"encoding/json"
	"net"
	"net/http"
	"os"
	"sync"

	"github.com/derWhity/kyabia/internal/ctxhelper"
	"github.com/derWhity/kyabia/internal/log"
	"github.com/derWhity/kyabia/internal/models"
	"github.com/derWhity/kyabia/internal/repos"
	"github.com/pkg/errors"
	"golang.org/x/net/context"
)

var (
	// ErrIllegalIP is the error returned when the provided string is no valid IP address
	ErrIllegalIP = MakeError(http.StatusBadRequest, "ILLEGAL_IP_ADDRESS", "Illegal IP address provided")
)

// ConfigService gives the authenticated user access to parts of the application's configuration
type ConfigService interface {
	// WhitelistedIPs returns the list of IP addresses that have been whitelisted for removing the restrictions guests
	// have when using Kyabia like limiting the total amount of wishes on the wishlist
	WhitelistedIPs(ctx context.Context) []string
	// AddToWhitelist adds an IP address to the list hosts without guest restrictions
	AddToWhitelist(ctx context.Context, ipAddr string) error
	// RemoveFromWhitelist removes an IP address from the list of hosts that have no guest restrictions
	RemoveFromWhitelist(ctx context.Context, ipAddr string) error
	// IsWhitelisted checks if the given IP address has been whitelisted
	IsWhitelisted(ipAddr string) bool
	// Load loads the application config from its default file location
	Load(ctx context.Context) error
	// LoadFromFile loads the configuration from the given JSON file and returns it
	LoadFromFile(ctx context.Context, filename string) error
	// Write writes the current application configuration to the default file name
	Write(ctx context.Context) error
	// WriteToFile writes the current application configuration to a JSON file
	WriteToFile(ctx context.Context, filename string) error
	// GetConfig retuns the current application configuration
	GetConfig(ctx context.Context) models.AppConfig
}

// -- ConfigService implementation -------------------------------------------------------------------------------------

// Simple index structure to speed up whitelist lookups
type whitelistIdx struct {
	sync.RWMutex
	data map[string]bool
}

type configService struct {
	configFilename string
	config         *models.AppConfig
	whitelist      *whitelistIdx
}

// NewConfigService creates a new configuration service instance with the given default file name
func NewConfigService(configFilename string) ConfigService {
	return &configService{
		configFilename: configFilename,
		whitelist: &whitelistIdx{
			data: make(map[string]bool),
		},
	}
}

func (s *configService) whitelistIdxToSlice() []string {
	ret := []string{}
	for item := range s.whitelist.data {
		ret = append(ret, item)
	}
	return ret
}

func (s *configService) buildWhitelistIdx(ctx context.Context) {
	logger := ctxhelper.Logger(ctx)
	logger.Info("Rebuilding index of whitelisted IPs...")
	s.whitelist.Lock()
	defer s.whitelist.Unlock()
	s.whitelist.data = make(map[string]bool)
	if s.config != nil {
		for _, ip := range s.config.Restrictions.IPWhitelist {
			s.whitelist.data[ip] = true
		}
	}
}

// WhitelistedIPs returns the list of IP addresses that have been whitelisted for removing the restrictions guests
// have when using Kyabia like limiting the total amount of wishes on the wishlist
func (s *configService) WhitelistedIPs(ctx context.Context) []string {
	s.whitelist.RLock()
	defer s.whitelist.RUnlock()
	return s.whitelistIdxToSlice()
}

// AddToWhitelist adds an IP address to the list hosts without guest restrictions
func (s *configService) AddToWhitelist(ctx context.Context, ipAddr string) error {
	logger := ctxhelper.Logger(ctx)
	if ip := net.ParseIP(ipAddr); ip == nil {
		return ErrIllegalIP
	}
	if s.IsWhitelisted(ipAddr) {
		// This IP is already whitelisted - just ignore
		return nil
	}
	logger.WithField(log.FldIP, ipAddr).Info("Adding IP address to whitelist")
	s.whitelist.Lock()
	defer s.whitelist.Unlock()
	s.whitelist.data[ipAddr] = true
	if s.config != nil {
		s.config.Restrictions.IPWhitelist = s.whitelistIdxToSlice()
	}
	return s.Write(ctx)
}

// RemoveFromWhitelist removes an IP address from the list of hosts that have no guest restrictions
func (s *configService) RemoveFromWhitelist(ctx context.Context, ipAddr string) error {
	if ip := net.ParseIP(ipAddr); ip == nil {
		return ErrIllegalIP
	}
	if !s.IsWhitelisted(ipAddr) {
		return repos.ErrEntityNotExisting
	}
	s.whitelist.Lock()
	defer s.whitelist.Unlock()
	delete(s.whitelist.data, ipAddr)
	if s.config != nil {
		s.config.Restrictions.IPWhitelist = s.whitelistIdxToSlice()
	}
	return s.Write(ctx)
}

// IsWhitelisted checks if the given IP address has been whitelisted
func (s *configService) IsWhitelisted(ipAddr string) bool {
	s.whitelist.RLock()
	defer s.whitelist.RUnlock()
	if _, ok := s.whitelist.data[ipAddr]; ok {
		return true
	}
	return false
}

// Load loads the application config from its default file location
func (s *configService) Load(ctx context.Context) error {
	return s.LoadFromFile(ctx, s.configFilename)
}

// LoadFromFile loads the configuration from the given JSON file and returns it
func (s *configService) LoadFromFile(ctx context.Context, filename string) error {
	logger := ctxhelper.Logger(ctx)
	logger.WithField(log.FldFile, filename).Info("Loading configuration file")
	conf, err := models.GetDefaultConfig()
	if err != nil {
		return errors.Wrap(err, "LoadFromFile: Failed to create default config")
	}
	f, err := os.Open(filename)
	if err != nil {
		return errors.Wrap(err, "LoadFromFile: cannot load configuration file")
	}
	defer f.Close()
	if err = json.NewDecoder(f).Decode(&conf); err != nil {
		return errors.Wrap(err, "LoadFromFile: Failed to decode configuration file")
	}
	s.config = conf
	s.buildWhitelistIdx(ctx)
	return nil
}

// Write writes the current application configuration to the default file name
func (s *configService) Write(ctx context.Context) error {
	return s.WriteToFile(ctx, s.configFilename)
}

// WriteToFile writes the current application configuration to a JSON file
func (s *configService) WriteToFile(ctx context.Context, filename string) error {
	logger := ctxhelper.Logger(ctx)
	logger.WithField(log.FldFile, filename).Info("Writing configuration file")
	f, err := os.Create(filename)
	if err != nil {
		return errors.Wrapf(err, "WriteToFile: Cannot open configuration file '%s' to write to", filename)
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	enc.SetIndent("", "    ")
	conf := s.GetConfig(ctx)
	if err := enc.Encode(&conf); err != nil {
		return errors.Wrap(err, "WriteToFile: Failed to serialize configuration data")
	}
	return nil
}

// GetConfig retuns the current application configuration
func (s *configService) GetConfig(ctx context.Context) models.AppConfig {
	var ret models.AppConfig
	if s.config != nil {
		ret = *s.config
	} else {
		if tmp, err := models.GetDefaultConfig(); err == nil {
			ret = *tmp
		}
	}
	return ret
}
