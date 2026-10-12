package cli

import (
	"encoding/json"
	"os"
	"testing"

	"gopkg.in/yaml.v3"
)

type ArtifactContent struct {
	// URI 版本文件路径
	URI string `json:"uri" binding:"required,uri,max=512" example:"s3://aiqa/store/workspaces/ws1/modelstores/ms1/models/model1/1"`

	// Description 版本描述
	Description string `json:"description" binding:"max=255" example:"artifact description"`

	// Alias 版本别名，如default, latest
	Alias []string `json:"alias" example:"[default]"`

	// Tags 版本标签 [key:value]
	Tags map[string]string `json:"tags"`

	// Metadata 版本基本信息
	Metadata any `json:"metadata"`
}

type CreateModelRequest struct {
	// WorkspaceID 工作区 id
	WorkspaceID string `uri:"workspaceID" example:"ws01"`

	// ModelStoreName 模型仓库名称
	ModelStoreName string `uri:"modelStoreName" example:"ms01"`

	// LocalName 系统名称
	LocalName string `json:"localName" binding:"required,localname" example:"model01"`

	// DisplayName 模型名称
	DisplayName string `json:"displayName" binding:"max=100" example:"模型01"`

	// Description 模型描述
	Description string `json:"description" binding:"max=255" example:"model description"`

	// Category 模型类别
	Category string `json:"category" binding:"required,max=105,category" example:"Image/OCR"`

	// ModelFormat 模型文件框架类型
	ModelFormats []string `json:"modelFormats" binding:"required,min=1" example:"[PaddlePaddle]"`

	// nolint
	// SchemaUri 模型对应的预测服务的接口文档地址
	SchemaUri string `json:"schemaUri" binding:"omitempty,uri"`

	// Artifact 模型版本
	Artifact ArtifactContent `json:"artifact"`
}

type ApplyModelRequest struct {
	CreateModelRequest `json:",inline"`
	LocationStyle      string `json:"locationStyle"`
}

func TestApply(t *testing.T) {
	f, err := os.OpenFile("testdata/apply.yaml", 0, os.ModePerm)
	if err != nil {
		t.Fatal(err)
	}

	var spec *CommandSpec
	d := yaml.NewDecoder(f)
	for d.Decode(&spec) == nil {
		if spec == nil {
			continue
		}

		var metadata ApplyModelRequest
		buf, err := json.Marshal(spec.Metadata)
		if err != nil {
			t.Fatal(err)
		}
		err = json.Unmarshal(buf, &metadata)
		if err != nil {
			continue
		}
		t.Log(metadata)
	}
	t.Log(spec)
}
