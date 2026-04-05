package backup

type RuntimeConfig struct {
	Rustic *RusticConfig `json:"rustic,omitempty"`
	Items  []RuntimeItem `json:"items"`
}

type RusticConfig struct {
	Repository string            `json:"repository"`
	Password   string            `json:"password"`
	Env        map[string]string `json:"env,omitempty"`
}

type RuntimeItem struct {
	Name     string   `json:"name"`
	Strategy string   `json:"strategy"`
	Service  string   `json:"service,omitempty"`
	Include  []string `json:"include,omitempty"`
	Provider string   `json:"provider"`
	Tags     []string `json:"tags,omitempty"`
	Retain   string   `json:"retain,omitempty"`
}
