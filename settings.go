package gotogen

type SettingItem struct {
	Name     string
	Options  []string
	Default  uint8
	Selected uint8
}

type SettingProvider interface {
	GetSettings() []*SettingItem
}
