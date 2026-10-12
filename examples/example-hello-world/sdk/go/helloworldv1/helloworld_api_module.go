package helloworldv1

import (
	"time"

	"github.com/FlyTOmeLight/server-go/sdk"
)

type Module struct {
	ID          string `json:"id" example:"id1"`                            // ID,唯一标识
	Name        string `json:"name" example:"name-1"`                       // 名称
	Description string `json:"description" example:"this is a description"` // 描述

	UserID    string `json:"userID" example:"user1"`       // 用户ID
	ProjectID string `json:"projectID" example:"project1"` // 项目ID

	CreatedAt time.Time `json:"createdAt"` // 创建时间
	UpdatedAt time.Time `json:"updatedAt"` // 更新时间
}

type ModuleID struct {
	ID string `json:"id" binding:"required,max=20" example:"id1"`
}

type CreateModuleRequest struct {
	ProjectID   string `json:"projectID" binding:"required,max=20" example:"project1"` // 项目ID
	Name        string `json:"name" binding:"required,max=32" example:"name-1"`        // 名称
	Description string `json:"description" binding:"max=200"`                          // 描述
}

type CreateModuleResponse = Module

type ListModuleRequest struct {
	sdk.PagingRequest

	ProjectID string `json:"projectID" binding:"omitempty,max=20" example:"project1"` // 项目ID
	Filter    string `json:"filter" binding:"omitempty,max=100" example:"name-1"`     // 搜索内容
}

type ListModuleResponse struct {
	TotalCount int      `json:"totalCount" example:"10"` // 总数
	Results    []Module `json:"result"`                  // 当前页结果
}

type GetModuleRequest = ModuleID

type GetModuleResponse = Module

type UpdateModuleRequest struct {
	ModuleID

	Description *string `json:"description" binding:"omitempty,max=200" example:"this is a description"` // 描述
}

type DeleteModuleRequest = ModuleID
