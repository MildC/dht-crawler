package torrent

type File struct {
	Path   []interface{} `json:"path"`
	Length int           `json:"length"`
}

type BitTorrent struct {
	InfoHash string `json:"infohash"`
	Name     string `json:"name"`
	Files    []File `json:"files,omitempty"`
	Length   int    `json:"length,omitempty"`
}
