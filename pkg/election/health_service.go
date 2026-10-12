package election

import (
	"context"
	"fmt"
	"net/http"

	"github.com/FlyTOmeLight/server-go/errors"
	"github.com/gin-gonic/gin"
)

// 用于检查当前节点是否是leader， 可用于有状态控制面服务的readiness probe，以避免从节点接收到请求
// 使用示例：
//
//	engine.GET("/election/isleader", election.IsLeader(e))
func IsLeader(e LeaderElection) gin.HandlerFunc {
	return func(c *gin.Context) {
		if e.IsLeader() {
			c.String(http.StatusOK, "leader")
			return
		}

		c.Abort()
		_ = c.Error(errors.NewError(http.StatusServiceUnavailable, "ServiceLoading", "Request is not allowed on standby node, current leader is"+e.GetLeader()))
	}
}

// 用于注册接口路由，实现从节点的readiness probe检查
type LeaderHealthService struct {
	e LeaderElection
}

func NewLeaderHealthService(e LeaderElection) *LeaderHealthService {
	return &LeaderHealthService{e: e}
}

func (s *LeaderHealthService) RegisterRoutes(group *gin.RouterGroup) error {
	if s.e == nil {
		return fmt.Errorf("leader election is required")
	}
	g := group.Group("/election")
	g.GET("/isleader", IsLeader(s.e))
	return nil
}

// The application owns the election; this service only exposes its readiness.
func (s *LeaderHealthService) Start(ctx context.Context) error { return ctx.Err() }
func (s *LeaderHealthService) Stop(context.Context) error      { return nil }
