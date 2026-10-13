package test

import (
	"testing"

	"github.com/FlyTOmeLight/server-go/sdk"
	"github.com/stretchr/testify/assert"

	"github.com/FlyTOmeLight/server-go/examples/example-service1/sdk/go/trainv1"
)

func TestDataset(t *testing.T) {
	t.Run("create", CreateDataset)
	t.Run("list", ListDataset)
	t.Run("update", UpdateDataset)
	t.Run("get", GetDataset)
	t.Run("delete", DeleteDataset)
}

func CreateDataset(t *testing.T) {
	dataset, err := client.CreateDataset(t.Context(), &trainv1.CreateDatasetRequest{
		ProjectName: sdkProjectName,
		LocalName:   sdkDatasetName,
		Description: "a desc",
	})
	assert.Nil(t, err)
	assert.NotNil(t, dataset)

	name := "projects/" + sdkProjectName + "/datasets/" + sdkDatasetName
	assert.Equal(t, name, dataset.Name)
}

func ListDataset(t *testing.T) {
	dataset, err := client.ListDataset(t.Context(), &trainv1.ListDatasetRequest{
		ProjectName:   sdkProjectName,
		PagingRequest: sdk.NewPagingRequest(),
	})
	assert.Nil(t, err)
	assert.NotNil(t, dataset)
	assert.Equal(t, 1, dataset.TotalCount)
	assert.Len(t, dataset.Results, 1)

	name := "projects/" + sdkProjectName + "/datasets/" + sdkDatasetName
	assert.Equal(t, dataset.Results[0].Name, name)
	assert.Equal(t, "a desc", dataset.Results[0].Description)
}

func GetDataset(t *testing.T) {
	name := "projects/" + sdkProjectName + "/datasets/" + sdkDatasetName
	dataset, err := client.GetDataset(t.Context(), trainv1.NewDatasetName(name))
	assert.Nil(t, err)
	assert.NotNil(t, dataset)
	assert.Equal(t, name, dataset.Name)
	assert.Equal(t, "new desc", dataset.Description)
}

func UpdateDataset(t *testing.T) {
	desc := "new desc"
	name := "projects/" + sdkProjectName + "/datasets/" + sdkDatasetName

	req := trainv1.NewUpdateDatasetRequest(name)
	req.Description = &desc
	err := client.UpdateDataset(t.Context(), req)
	assert.Nil(t, err)
}

func DeleteDataset(t *testing.T) {
	name := "projects/" + sdkProjectName + "/datasets/" + sdkDatasetName
	err := client.DeleteDataset(t.Context(), trainv1.NewDatasetName(name))
	assert.Nil(t, err)
}
