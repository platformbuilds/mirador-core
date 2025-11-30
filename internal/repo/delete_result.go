package repo

// DeleteStoreResult describes the outcome for a single storage/cache layer.
type DeleteStoreResult struct {
	Found   bool   `json:"found"`
	Deleted bool   `json:"deleted"`
	Error   string `json:"error,omitempty"`
}

// DeleteResult is the structured result returned by DeleteKPI.
type DeleteResult struct {
	Weaviate DeleteStoreResult `json:"weaviate"`
	Valkey   DeleteStoreResult `json:"valkey"`
	Bleve    DeleteStoreResult `json:"bleve"`
}
