package service

import (
	"context"
	"github.com/gin-gonic/gin"
)

type ExampleService struct{}

func NewExampleService() *ExampleService                               { return &ExampleService{} }
func (s *ExampleService) RegisterRoutes(router *gin.RouterGroup) error { return nil }
func (s *ExampleService) Start(context.Context) error                  { return nil }
func (s *ExampleService) Stop(context.Context) error                   { return nil }
