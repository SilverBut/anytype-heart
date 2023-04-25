package csv

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/anytypeio/go-anytype-middleware/core/block/editor/smartblock"
	"github.com/anytypeio/go-anytype-middleware/core/block/process"
	"github.com/anytypeio/go-anytype-middleware/pb"
	"github.com/anytypeio/go-anytype-middleware/pkg/lib/bundle"
	"github.com/anytypeio/go-anytype-middleware/pkg/lib/pb/model"
	"github.com/anytypeio/go-anytype-middleware/util/pbtypes"
)

func TestCsv_GetSnapshotsEmptyFile(t *testing.T) {
	csv := CSV{}
	p := process.NewProgress(pb.ModelProcess_Import)
	sn, err := csv.GetSnapshots(&pb.RpcObjectImportRequest{
		Params: &pb.RpcObjectImportRequestParamsOfCsvParams{
			CsvParams: &pb.RpcObjectImportRequestCsvParams{Path: []string{"testdata/test.csv"}},
		},
		Type: pb.RpcObjectImportRequest_Csv,
		Mode: pb.RpcObjectImportRequest_IGNORE_ERRORS,
	}, p)

	assert.NotNil(t, sn)
	assert.Len(t, sn.Snapshots, 2) //test + root collection
	assert.Contains(t, sn.Snapshots[0].FileName, "test.csv")
	assert.Len(t, pbtypes.GetStringList(sn.Snapshots[0].Snapshot.Data.Collections, smartblock.CollectionStoreKey), 0)
	assert.NotEmpty(t, sn.Snapshots[1].Snapshot.Data.ObjectTypes) //empty collection
	assert.Equal(t, sn.Snapshots[0].Snapshot.Data.ObjectTypes[0], bundle.TypeKeyCollection.URL())

	assert.Contains(t, sn.Snapshots[1].FileName, rootCollectionName)
	assert.NotEmpty(t, sn.Snapshots[1].Snapshot.Data.ObjectTypes)
	assert.Equal(t, sn.Snapshots[1].Snapshot.Data.ObjectTypes[0], bundle.TypeKeyCollection.URL())
	assert.Len(t, pbtypes.GetStringList(sn.Snapshots[1].Snapshot.Data.Collections, smartblock.CollectionStoreKey), 1)
	assert.Nil(t, err)
}

func TestCsv_GetSnapshots(t *testing.T) {
	csv := CSV{}
	p := process.NewProgress(pb.ModelProcess_Import)
	sn, err := csv.GetSnapshots(&pb.RpcObjectImportRequest{
		Params: &pb.RpcObjectImportRequestParamsOfCsvParams{
			CsvParams: &pb.RpcObjectImportRequestCsvParams{Path: []string{"testdata/Journal.csv"}},
		},
		Type: pb.RpcObjectImportRequest_Csv,
		Mode: pb.RpcObjectImportRequest_IGNORE_ERRORS,
	}, p)

	assert.Nil(t, err)

	assert.NotNil(t, sn)
	assert.Len(t, sn.Snapshots, 4) //test + root collection + 2 objects in Journal.csv
	assert.Contains(t, sn.Snapshots[0].FileName, "Journal.csv")
	assert.Len(t, pbtypes.GetStringList(sn.Snapshots[0].Snapshot.Data.Collections, smartblock.CollectionStoreKey), 2) //2 objects

	var found bool
	for _, snapshot := range sn.Snapshots {
		if snapshot.FileName == rootCollectionName {
			found = true
			assert.NotEmpty(t, snapshot.Snapshot.Data.ObjectTypes)
			assert.Equal(t, snapshot.Snapshot.Data.ObjectTypes[0], bundle.TypeKeyCollection.URL())
			assert.Len(t, pbtypes.GetStringList(snapshot.Snapshot.Data.Collections, smartblock.CollectionStoreKey), 3) //all objects
		}
	}

	assert.True(t, found)
}

func TestCsv_GetSnapshotsTable(t *testing.T) {
	csv := CSV{}
	p := process.NewProgress(pb.ModelProcess_Import)
	sn, err := csv.GetSnapshots(&pb.RpcObjectImportRequest{
		Params: &pb.RpcObjectImportRequestParamsOfCsvParams{
			CsvParams: &pb.RpcObjectImportRequestCsvParams{
				Path: []string{"testdata/Journal.csv"},
				Mode: pb.RpcObjectImportRequestCsvParams_TABLE,
			},
		},
		Type: pb.RpcObjectImportRequest_Csv,
		Mode: pb.RpcObjectImportRequest_IGNORE_ERRORS,
	}, p)

	assert.Nil(t, err)

	assert.NotNil(t, sn)
	assert.Len(t, sn.Snapshots, 2) //1 page with table + root collection
	assert.Contains(t, sn.Snapshots[0].FileName, "Journal.csv")

	var found bool
	for _, bl := range sn.Snapshots[0].Snapshot.Data.Blocks {
		if _, ok := bl.Content.(*model.BlockContentOfTable); ok {
			found = true
		}
	}
	assert.True(t, found)
}