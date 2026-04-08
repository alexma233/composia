package backup

type RuntimeConfig struct {
	Rustic *RusticConfig `json:"rustic,omitempty"`
	Items  []RuntimeItem `json:"items"`
}

type RusticConfig struct {
	ServiceName    string `json:"service_name"`
	ServiceDir     string `json:"service_dir"`
	ComposeService string `json:"compose_service"`
	Profile        string `json:"profile,omitempty"`
	NodeID         string `json:"node_id"`
}

type RuntimeItem struct {
	Name     string   `json:"name"`
	Strategy string   `json:"strategy"`
	Service  string   `json:"service,omitempty"`
	Include  []string `json:"include,omitempty"`
	Provider string   `json:"provider"`
	Tags     []string `json:"tags,omitempty"`
}
