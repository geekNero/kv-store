package spec

const (
	KeyStorePath = "/keystore/"
)

type PutRequest struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}
