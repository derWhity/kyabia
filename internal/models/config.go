package models

import (
	"path"

	"github.com/kardianos/osext"
)

// AppConfig is the application's main configuration structure
type AppConfig struct {
	// The directory where Kyabia stores all of its data - defaults to the /data subdirectory of the folder, the
	// Kyabia executable resides in
	DataDir string `json:"dataDir"`
	// The credentials for the default user account that is created on startup
	DefaultUser *DefaultUserConfig `json:"defaultUser"`
	// The IP address to listen at - including the port number
	ListenAddress string `json:"listenAddress"`
	// The restrictions for guests working with Kyabia
	Restrictions GuestRestrictionConfig `json:"restrictions"`
}

// The DefaultUserConfig struct configures the default user that can log in
// In a later version, this will be replaced by a full user management
type DefaultUserConfig struct {
	Name     string `json:"name"`
	Password string `json:"password"`
}

// GuestRestrictionConfig is the configuration for restricting some aspects of Kyabia for guest users
type GuestRestrictionConfig struct {
	// NumWishesFromSameIP is the number of unplayed wishes from the same IP address allowed in the main playlist
	NumWishesFromSameIP uint `json:"wishesFromSameIP"`
	// Can be set to `true` to allow the same video to be wished twice
	AllowDuplicateWishes bool `json:"allowDuplicateWishes"`
	// A list of IP addresses whitelisted. Guests from these IPs will have the restrictions lifted
	IPWhitelist []string `json:"ipWhitelist"`
}

// GetDefaultConfig returns the default configuration values for the application
func GetDefaultConfig() (*AppConfig, error) {
	execDir, err := osext.ExecutableFolder()
	if err != nil {
		return nil, err
	}
	return &AppConfig{
		DataDir: path.Join(execDir, "data"),
		DefaultUser: &DefaultUserConfig{
			Name:     "admin",
			Password: "changeme",
		},
		Restrictions: GuestRestrictionConfig{
			NumWishesFromSameIP: 2,
			IPWhitelist:         []string{},
		},
		ListenAddress: ":3000",
	}, nil
}
