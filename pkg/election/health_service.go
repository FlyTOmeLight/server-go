package election

import (
	"net/http"

	"github.com/FlyTOmeLight/server-go/errors"
	core "github.com/FlyTOmeLight/server-go/pkg"
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
	engine *core.Engine
	e      LeaderElection
}

func NewLeaderHealthService(e LeaderElection) *LeaderHealthService {
	return &LeaderHealthService{
		engine: nil,
		e:      e,
	}
}

func (s *LeaderHealthService) Initialize(e *core.Engine) {
	s.engine = e
}

func (s *LeaderHealthService) StartAsync() {
	g := s.engine.Group("/election")
	g.GET("/isleader", IsLeader(s.e))
}

// StopAsync 异步的停止，无阻塞操作。
func (s *LeaderHealthService) StopAsync() {
}
