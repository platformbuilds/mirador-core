package repo

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/platformbuilds/mirador-core/internal/models"
	"github.com/platformbuilds/mirador-core/internal/weavstore"
)

// mock store that simulates missing KPIDefinition class in Weaviate for ListKPIs
type mockKPIStoreMissing struct{}

func (m *mockKPIStoreMissing) CreateOrUpdateKPI(ctx context.Context, k *weavstore.KPIDefinition) (*weavstore.KPIDefinition, string, error) {
	return nil, "", nil
}
func (m *mockKPIStoreMissing) GetKPI(ctx context.Context, id string) (*weavstore.KPIDefinition, error) {
	return nil, weavstore.ErrKPIDefinitionClassMissing
}
func (m *mockKPIStoreMissing) ListKPIs(ctx context.Context, req *weavstore.KPIListRequest) ([]*weavstore.KPIDefinition, int64, error) {
	return nil, 0, weavstore.ErrKPIDefinitionClassMissing
}
func (m *mockKPIStoreMissing) SearchKPIs(ctx context.Context, req *weavstore.KPISearchRequest) ([]*weavstore.KPISearchResult, int64, error) {
	return nil, 0, weavstore.ErrKPIDefinitionClassMissing
}
func (m *mockKPIStoreMissing) DeleteKPI(ctx context.Context, id string) error {
	return weavstore.ErrKPIDefinitionClassMissing
}

func Test_ListKPIs_TreatMissingSchemaAsEmpty(t *testing.T) {
	ctx := context.Background()

	mock := &mockKPIStoreMissing{}
	r := NewDefaultKPIRepo(mock, nil, nil, nil)

	kpis, total, err := r.ListKPIs(ctx /*req*/, models.KPIListRequest{})
	require.NoError(t, err)
	require.Empty(t, kpis)
	require.EqualValues(t, 0, total)
}
