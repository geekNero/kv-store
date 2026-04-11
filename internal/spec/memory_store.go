package spec

const (
	KeyStorePath = "/keystore/"
)

type PutRequest struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type WALRequest struct {
	Key       string `json:"key"`
	Value     string `json:"value"`
	Operation string `json:"op"`
	Hash      uint32 `json:"hash,omitempty"`
}

type SSTEntry struct {
	Key       string  `json:"key"`
	Value     *string `json:"value,omitempty"`
	Tombstone bool    `json:"deleted,omitempty"`
}

type SSTMetaData struct {
	Name     string `json:"sst_name"`
	FirstKey string `json:"first_key"`
	LastKey  string `json:"second_key"`
}
