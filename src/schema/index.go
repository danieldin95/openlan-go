package schema

type Index struct {
	Version   Version     `json:"version"`
	Worker    Worker      `json:"worker"`
	Points    []Point     `json:"points"`
	Links     []Link      `json:"links"`
	Neighbors []Neighbor  `json:"neighbors"`
	OnLines   []OnLine    `json:"online"`
	Network   []Network   `json:"network"`
	Clients   []VPNClient `json:"clients"`
}

type Ctrl struct {
	Url   string `json:"url"`
	Token string `json:"token"`
}
