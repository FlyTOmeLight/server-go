package trainv1

import (
	"regexp"
	"time"

	"github.com/FlyTOmeLight/server-go/sdk"
)

var DatasetNameRegex = regexp.MustCompile("^projects/(?P<projectName>.+?)/datasets/(?P<datasetName>.+?)$")

type FileFormat string

const (
	FileFormatTFRecord FileFormat = "TFRecord"
	FileFormatFile     FileFormat = "File"
)

type Dataset struct {
	Name        string `json:"name"`
	LocalName   string `json:"localName"`
	Description string `json:"description"`

	FileFormat FileFormat        `json:"fileFormat"`
	Tags       map[string]string `json:"tags"`

	ProjectName string `json:"projectName"`

	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type DatasetName struct {
	ProjectName string `uri:"projectName"`
	LocalName   string `uri:"localName"`
}

func GetDatasetName(workspaceID, projectName, localName string) string {
	name := "projects/" + projectName + "/datasets/" + localName

	return name
}

func (d *DatasetName) GetName() string {
	return "projects/" + d.ProjectName + "/datasets/" + d.LocalName
}

func NewDatasetName(name string) *DatasetName {
	m := DatasetNameRegex.FindStringSubmatch(name)
	if len(m) < DatasetNameRegex.NumSubexp() {
		return nil
	}

	return &DatasetName{
		ProjectName: m[DatasetNameRegex.SubexpIndex("projectName")],
		LocalName:   m[DatasetNameRegex.SubexpIndex("datasetName")],
	}
}

type CreateDatasetRequest struct {
	LocalName   string `json:"localName" binding:"localname,required"`
	Description string `json:"description" binding:"max=255"`
	FileFormat  string `json:"fileFormat" binding:"omitempty,max=64,oneof=Parquet Zip Tar HDF5 Petastorm TFRecord File Folder"`
	ProjectName string `uri:"projectName"`
}

func (d *CreateDatasetRequest) GetName() string {
	return "projects/" + d.ProjectName + "/datasets/" + d.LocalName
}

type CreateDatasetResponse = Dataset

type ListDatasetRequest struct {
	sdk.PagingRequest

	ProjectName string `uri:"projectName"`
	Filter      string `form:"filter"`
}

type ListDatasetResponse struct {
	TotalCount int       `json:"totalCount"`
	Results    []Dataset `json:"result"`
}

type GetDatasetRequest = DatasetName

type GetDatasetResponse = Dataset

type UpdateDatasetRequest struct {
	DatasetName
	Description *string `json:"description"`
}

func NewUpdateDatasetRequest(name string) *UpdateDatasetRequest {
	datasetName := NewDatasetName(name)
	if datasetName == nil {
		return nil
	}
	return &UpdateDatasetRequest{
		DatasetName: *datasetName,
	}
}

type DeleteDatasetRequest = DatasetName
