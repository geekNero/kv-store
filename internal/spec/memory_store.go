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
}
