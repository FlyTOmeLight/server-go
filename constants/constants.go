package constants

// http header keys
const (
	HeaderRequestID     = "Request-ID"
	HeaderContentType   = "Content-Type"
	HeaderAuthorization = "Authorization"

	HeaderImpersonateTenantID  = "x-impersonate-target-tenant-id"
	HeaderImpersonateUserID    = "x-impersonate-target-user-id"
	HeaderImpersonateProjectID = "x-impersonate-target-project-id"

	HeaderTenantID  = "x-forge-tenant-id"
	HeaderUserID    = "x-forge-user-id"
	HeaderProjectID = "x-forge-project-id"

	ContentTypeJSON = "application/json"
)

// http url query key
const (
	QueryUserID      = "userID"
	QueryClientToken = "clientToken"
)

// core.Context keys
const (
	KeyRequestID = "RequestID"

	KeyUserID    = "UserID"    // auth后获得的UserID，不可修改
	KeyTenantID  = "TenantID"  // auth后获得的TenantID，不可修改
	KeyProjectID = "ProjectID" // auth后获得的ProjectID，不可修改
	KeyUserName  = "UserName"  // auth后获得的用户名称
	KeyUserRole  = "UserRole"  // 用户角色
	KeyAuthMode  = "AuthMode"  // 用户认证方式

	KeyImpersonateOriginTenantID  = "ImpersonateOriginTenantID"  // A impersonate成B访问服务时的tenantID
	KeyImpersonateOriginUserID    = "ImpersonateOriginUserID"    // A impersonate成B访问服务时的userID
	KeyImpersonateOriginProjectID = "ImpersonateOriginProjectID" // A impersonate成B访问服务时的projectID
)

const (
	SizeMB = 1 * 1024 * 1024
)
