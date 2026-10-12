package naming

import (
	"regexp"

	"github.com/FlyTOmeLight/server-go/errors"
)

var nameRegex1 = regexp.MustCompile("^workspaces/(?P<workspaceID>.+?)/(?P<parentType>.+?)s/(?P<parentName>.+?)/(?P<objectType>.+?)s/(.+?)$")
var nameRegex2 = regexp.MustCompile("^(?P<parentType>.+?)s/(?P<parentName>.+?)/(?P<objectType>.+?)s/(.+?)$")

// workspaces/a/devicehubs/b/devices/c/modules/1
var nameRegex3 = regexp.MustCompile(
	"^workspaces/(?P<workspaceID>.+?)/" +
		"(?P<parentType>.+?)s/(?P<parentName>.+?)/" +
		"(?P<objectType>.+?)s/(?P<objectName>.+?)/" +
		"(?P<subObjectType>.+?)s/(?P<subObjectName>.+?)")

type Naming struct {
	// ObjectType 数据类型，如model、dataset等
	ObjectType string `json:"objectType" example:"model"`

	// ObjectName 数据全名
	ObjectName string `json:"objectName" example:"workspaces/ws1/modelstores/ms1/models/model1"`

	// ParentType 数据所属的父级类型，如project、modelstore
	ParentType string `json:"parentType,omitempty" example:"modelstore"`

	// ParentName 数据所属的父级全名
	ParentName string `json:"parentName,omitempty" example:"workspaces/ws1/modelstores/ms1"`

	// WorkspaceID 工作区 id
	WorkspaceID string `json:"workspaceID" example:"ws1"`
}

type Data struct {
	ObjectType  string `gorm:"type:varchar(20);default:'';NOT NULL"`
	ObjectName  string `gorm:"type:varchar(210);default:'';NOT NULL;index:idx_object_name"`
	ParentType  string `gorm:"type:varchar(20);default:'';NOT NULL"`
	ParentName  string `gorm:"type:varchar(200);default:'';NOT NULL"`
	WorkspaceID string `gorm:"column:workspace_id;type:varchar(64);default:'';NOT NULL"`
}

func New(n Naming) *Naming {
	if n.ObjectName == "" {
		return nil
	}

	current := NewByObjectName(n.ObjectName)
	if n.ObjectType != "" {
		current.ObjectType = n.ObjectType
	}
	if n.ParentName != "" {
		current.ParentName = n.ParentName
	}
	if n.ParentType != "" {
		current.ParentType = n.ParentType
	}
	if n.WorkspaceID != "" {
		current.WorkspaceID = n.WorkspaceID
	}

	return current
}

func NewByObjectName(objectName string) *Naming {
	n := getNameRegex3(objectName)
	if n == nil {
		n = getNameRegex1(objectName)
	}
	if n == nil {
		n = getNameRegex2(objectName)
	}
	if n == nil {
		return &Naming{ObjectName: objectName}
	}

	return n
}

func Check(objectName string) error {
	n := getNameRegex3(objectName)
	if n != nil {
		return nil
	}
	n = getNameRegex1(objectName)
	if n != nil {
		return nil
	}
	n = getNameRegex2(objectName)
	if n != nil {
		return nil
	}

	return errors.NewInvalidParameter("objectName is invalid")
}

func getNameRegex1(objectName string) *Naming {
	m := nameRegex1.FindStringSubmatch(objectName)
	if len(m) < nameRegex1.NumSubexp() {
		return nil
	}
	n := Naming{
		ObjectType:  m[nameRegex1.SubexpIndex("objectType")],
		ObjectName:  objectName,
		ParentType:  m[nameRegex1.SubexpIndex("parentType")],
		WorkspaceID: m[nameRegex1.SubexpIndex("workspaceID")],
	}

	n.ParentName = "workspaces/" + n.WorkspaceID + "/" + n.ParentType + "s/" + m[nameRegex1.SubexpIndex("parentName")]
	return &n
}

func getNameRegex2(objectName string) *Naming {
	m := nameRegex2.FindStringSubmatch(objectName)
	if len(m) < nameRegex2.NumSubexp() {
		return nil
	}

	n := Naming{
		ObjectType:  m[nameRegex2.SubexpIndex("objectType")],
		ObjectName:  objectName,
		ParentType:  m[nameRegex2.SubexpIndex("parentType")],
		ParentName:  m[nameRegex2.SubexpIndex("parentName")],
		WorkspaceID: m[nameRegex2.SubexpIndex("parentName")],
	}

	if n.ParentType == "workspace" {
		n.WorkspaceID = n.ParentName
		n.ParentName = "workspaces/" + n.WorkspaceID
		return &n
	}

	n.ParentName = n.ParentType + "s/" + n.ParentName
	return &n
}

func getNameRegex3(objectName string) *Naming {
	m := nameRegex3.FindStringSubmatch(objectName)
	if len(m) < nameRegex3.NumSubexp() {
		return nil
	}
	if subObjectType := m[nameRegex3.SubexpIndex("subObjectType")]; subObjectType == "" || subObjectType == "version" {
		return nil
	}

	n := Naming{
		ObjectType: m[nameRegex3.SubexpIndex("subObjectType")],
		ObjectName: objectName,
		ParentType: m[nameRegex3.SubexpIndex("objectType")],
		ParentName: m[nameRegex3.SubexpIndex("parentType")] +
			"s/" + m[nameRegex3.SubexpIndex("parentName")] + "/" +
			m[nameRegex3.SubexpIndex("objectType")] + "s/" + m[nameRegex3.SubexpIndex("objectName")],
		WorkspaceID: m[nameRegex3.SubexpIndex("workspaceID")],
	}

	n.ParentName = "workspaces/" + n.WorkspaceID + "/" + n.ParentName

	return &n
}

// workspaces/a/devicehubs/b/devices/c/modules/1
