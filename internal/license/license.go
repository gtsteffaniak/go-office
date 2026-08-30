package license

// Response is sent to the browser over the coauthoring channel on connect.
// Shape matches ONLYOFFICE Document Server license messages (community/Euro-Office).
type Response struct {
	Type    string `json:"type"`
	License Info   `json:"license"`
}

// Info describes editor capabilities exposed to sdkjs.
type Info struct {
	Type            int    `json:"type"`
	Mode            int    `json:"mode"`
	Branding        bool   `json:"branding"`
	Customization   bool   `json:"customization"`
	AdvancedAPI     bool   `json:"advancedApi"`
	Connections     int    `json:"connections"`
	ConnectionsView int    `json:"connectionsView"`
	UsersCount      int    `json:"usersCount"`
	UsersViewCount  int    `json:"usersViewCount"`
	HasLicense      bool   `json:"hasLicense"`
	BuildDate       string `json:"buildDate,omitempty"`
	BuildVersion    string `json:"buildVersion,omitempty"`
	CustomerID      string `json:"customerId,omitempty"`
	Alias           string `json:"alias,omitempty"`
	Multitenancy    bool   `json:"multitenancy"`
	Light           bool   `json:"light"`
	Plugins         bool   `json:"plugins"`
	PackageType     int    `json:"packageType"`
	EndDate         any    `json:"endDate"`
	UsersExpire     int    `json:"usersExpire,omitempty"`
}

// Permissive returns a license payload suitable for Euro-Office / community use.
func Permissive(buildVersion string) Response {
	return Response{
		Type: "license",
		License: Info{
			Type:            1,
			Mode:            0,
			Branding:        false,
			Customization:   true,
			AdvancedAPI:     true,
			Connections:     9999,
			ConnectionsView: 9999,
			UsersCount:      9999,
			UsersViewCount:  9999,
			HasLicense:      true,
			BuildVersion:    buildVersion,
			Alias:           "go-office",
			Multitenancy:    false,
			Light:           false,
			Plugins:         true,
			PackageType:     0,
			EndDate:         nil,
			UsersExpire:     86400,
		},
	}
}
