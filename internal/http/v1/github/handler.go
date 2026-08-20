package github

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/janisto/echo-observability/v2"
	"github.com/labstack/echo/v5"
	"go.uber.org/zap"

	"github.com/janisto/echo-playground/internal/platform/pagination"
	"github.com/janisto/echo-playground/internal/platform/request"
	"github.com/janisto/echo-playground/internal/platform/respond"
	githubsvc "github.com/janisto/echo-playground/internal/service/github"
)

const (
	listOwnerRepositoriesOperation = "listGitHubOwnerRepositories"
	listActivityOperation          = "listGitHubRepositoryActivity"
	listTagsOperation              = "listGitHubRepositoryTags"
	rateLimitResetHeader           = "X-Ratelimit-Reset"
)

func Register(group *echo.Group, service githubsvc.Service) {
	negotiation := respond.SuccessNegotiation(false)
	group.GET("/github/owners/:owner", getOwner(service), negotiation)
	group.GET("/github/owners/:owner/repos", listOwnerRepositories(service), negotiation)
	group.GET("/github/repos/:owner/:repo", getRepository(service), negotiation)
	group.GET("/github/repos/:owner/:repo/activity", listRepositoryActivity(service), negotiation)
	group.GET("/github/repos/:owner/:repo/languages", listRepositoryLanguages(service), negotiation)
	group.GET("/github/repos/:owner/:repo/tags", listRepositoryTags(service), negotiation)
}

// getOwner gets one public GitHub owner through the fixed anonymous client.
//
//	@Summary		Get a public GitHub owner
//	@ID				getGitHubOwner
//	@Description	Projects a public owner from the fixed anonymous GitHub origin. The query string is closed.
//	@Tags			github
//	@Produce		json,application/cbor
//	@Param			X-Request-ID	header		string	false	"Optional request correlation value"	minlength(1)	maxlength(128)
//	@Param			owner			path		string	true	"GitHub owner"							minlength(1)	maxlength(39)
//	@Success		200				{object}	Owner
//	@Failure		400				{object}	respond.ProblemDetails
//	@Failure		404				{object}	respond.ProblemDetails
//	@Failure		406				{object}	respond.ProblemDetails
//	@Failure		422				{object}	respond.ProblemDetails
//	@Failure		429				{object}	respond.ProblemDetails
//	@Failure		500				{object}	respond.ProblemDetails
//	@Failure		502				{object}	respond.ProblemDetails
//	@Failure		504				{object}	respond.ProblemDetails
//	@Router			/v1/github/owners/{owner} [get]
func getOwner(service githubsvc.Service) echo.HandlerFunc {
	return func(c *echo.Context) error {
		owner, err := pointInput(c, false)
		if err != nil {
			return err
		}
		result, err := service.GetOwner(c.Request().Context(), owner)
		if err != nil {
			return mapServiceError(c, "get owner", err)
		}
		return respond.Negotiate(c, http.StatusOK, ownerModel(result))
	}
}

// listOwnerRepositories lists public repositories for one owner.
//
//	@Summary		List public GitHub owner repositories
//	@ID				listGitHubOwnerRepositories
//	@Description	Returns a strictly projected, cursor-paginated public repository page.
//	@Tags			github
//	@Produce		json,application/cbor
//	@Param			X-Request-ID	header		string	false	"Optional request correlation value"	minlength(1)	maxlength(128)
//	@Param			owner			path		string	true	"GitHub owner"							minlength(1)	maxlength(39)
//	@Param			limit			query		int		false	"Page size"								default(20)		minimum(1)	maximum(100)
//	@Param			cursor			query		string	false	"Opaque scoped cursor"					minlength(1)	maxlength(2048)
//	@Success		200				{object}	RepositoryPage
//	@Failure		400				{object}	respond.ProblemDetails
//	@Failure		404				{object}	respond.ProblemDetails
//	@Failure		406				{object}	respond.ProblemDetails
//	@Failure		422				{object}	respond.ProblemDetails
//	@Failure		429				{object}	respond.ProblemDetails
//	@Failure		500				{object}	respond.ProblemDetails
//	@Failure		502				{object}	respond.ProblemDetails
//	@Failure		504				{object}	respond.ProblemDetails
//	@Header			200				{string}	Link	"Optional RFC 8288 pagination links"
//	@Router			/v1/github/owners/{owner}/repos [get]
func listOwnerRepositories(service githubsvc.Service) echo.HandlerFunc {
	return func(c *echo.Context) error {
		owner, err := pathInput(c, false)
		if err != nil {
			return err
		}
		limit, cursor, err := pageInput(c, pagination.Scope{
			Operation: listOwnerRepositoriesOperation,
			Owner:     owner,
		})
		if err != nil {
			return err
		}
		result, err := service.ListOwnerRepositories(c.Request().Context(), owner, limit, cursor)
		if err != nil {
			return mapServiceError(c, "list owner repositories", err)
		}
		setPageLink(c, "/v1/github/owners/"+owner+"/repos", limit, result.NextCursor, result.PrevCursor)
		return respond.Negotiate(c, http.StatusOK, repositoryPageModel(result.Entries))
	}
}

// getRepository gets one public GitHub repository.
//
//	@Summary		Get a public GitHub repository
//	@ID				getGitHubRepository
//	@Description	Projects one public repository from the fixed anonymous GitHub origin. The query string is closed.
//	@Tags			github
//	@Produce		json,application/cbor
//	@Param			X-Request-ID	header		string	false	"Optional request correlation value"	minlength(1)	maxlength(128)
//	@Param			owner			path		string	true	"GitHub owner"							minlength(1)	maxlength(39)
//	@Param			repo			path		string	true	"GitHub repository"						minlength(1)	maxlength(100)
//	@Success		200				{object}	Repository
//	@Failure		400				{object}	respond.ProblemDetails
//	@Failure		404				{object}	respond.ProblemDetails
//	@Failure		406				{object}	respond.ProblemDetails
//	@Failure		422				{object}	respond.ProblemDetails
//	@Failure		429				{object}	respond.ProblemDetails
//	@Failure		500				{object}	respond.ProblemDetails
//	@Failure		502				{object}	respond.ProblemDetails
//	@Failure		504				{object}	respond.ProblemDetails
//	@Router			/v1/github/repos/{owner}/{repo} [get]
func getRepository(service githubsvc.Service) echo.HandlerFunc {
	return func(c *echo.Context) error {
		owner, repository, err := repositoryInput(c, false)
		if err != nil {
			return err
		}
		result, err := service.GetRepository(c.Request().Context(), owner, repository)
		if err != nil {
			return mapServiceError(c, "get repository", err)
		}
		return respond.Negotiate(c, http.StatusOK, repositoryModel(result))
	}
}

// listRepositoryActivity lists public GitHub repository activity.
//
//	@Summary		List public GitHub repository activity
//	@ID				listGitHubRepositoryActivity
//	@Description	Returns a newest-first, cursor-paginated public activity page.
//	@Tags			github
//	@Produce		json,application/cbor
//	@Param			X-Request-ID	header		string	false	"Optional request correlation value"	minlength(1)	maxlength(128)
//	@Param			owner			path		string	true	"GitHub owner"							minlength(1)	maxlength(39)
//	@Param			repo			path		string	true	"GitHub repository"						minlength(1)	maxlength(100)
//	@Param			limit			query		int		false	"Page size"								default(20)		minimum(1)	maximum(100)
//	@Param			cursor			query		string	false	"Opaque scoped cursor"					minlength(1)	maxlength(2048)
//	@Success		200				{object}	ActivityPage
//	@Failure		400				{object}	respond.ProblemDetails
//	@Failure		404				{object}	respond.ProblemDetails
//	@Failure		406				{object}	respond.ProblemDetails
//	@Failure		422				{object}	respond.ProblemDetails
//	@Failure		429				{object}	respond.ProblemDetails
//	@Failure		500				{object}	respond.ProblemDetails
//	@Failure		502				{object}	respond.ProblemDetails
//	@Failure		504				{object}	respond.ProblemDetails
//	@Header			200				{string}	Link	"Optional RFC 8288 pagination links"
//	@Router			/v1/github/repos/{owner}/{repo}/activity [get]
func listRepositoryActivity(service githubsvc.Service) echo.HandlerFunc {
	return func(c *echo.Context) error {
		owner, repository, err := repositoryInput(c, true)
		if err != nil {
			return err
		}
		limit, cursor, err := pageInput(c, pagination.Scope{
			Operation:  listActivityOperation,
			Owner:      owner,
			Repository: repository,
		})
		if err != nil {
			return err
		}
		result, err := service.ListRepositoryActivity(c.Request().Context(), owner, repository, limit, cursor)
		if err != nil {
			return mapServiceError(c, "list repository activity", err)
		}
		setPageLink(
			c,
			"/v1/github/repos/"+owner+"/"+repository+"/activity",
			limit,
			result.NextCursor,
			result.PrevCursor,
		)
		return respond.Negotiate(c, http.StatusOK, activityPageModel(result.Entries))
	}
}

// listRepositoryLanguages lists public GitHub repository languages.
//
//	@Summary		List public GitHub repository languages
//	@ID				listGitHubRepositoryLanguages
//	@Description	Returns the strictly projected language byte counts. The query string is closed.
//	@Tags			github
//	@Produce		json,application/cbor
//	@Param			X-Request-ID	header		string	false	"Optional request correlation value"	minlength(1)	maxlength(128)
//	@Param			owner			path		string	true	"GitHub owner"							minlength(1)	maxlength(39)
//	@Param			repo			path		string	true	"GitHub repository"						minlength(1)	maxlength(100)
//	@Success		200				{object}	Languages
//	@Failure		400				{object}	respond.ProblemDetails
//	@Failure		404				{object}	respond.ProblemDetails
//	@Failure		406				{object}	respond.ProblemDetails
//	@Failure		422				{object}	respond.ProblemDetails
//	@Failure		429				{object}	respond.ProblemDetails
//	@Failure		500				{object}	respond.ProblemDetails
//	@Failure		502				{object}	respond.ProblemDetails
//	@Failure		504				{object}	respond.ProblemDetails
//	@Router			/v1/github/repos/{owner}/{repo}/languages [get]
func listRepositoryLanguages(service githubsvc.Service) echo.HandlerFunc {
	return func(c *echo.Context) error {
		owner, repository, err := repositoryInput(c, false)
		if err != nil {
			return err
		}
		result, err := service.ListRepositoryLanguages(c.Request().Context(), owner, repository)
		if err != nil {
			return mapServiceError(c, "list repository languages", err)
		}
		return respond.Negotiate(c, http.StatusOK, languagesModel(result))
	}
}

// listRepositoryTags lists public GitHub repository tags.
//
//	@Summary		List public GitHub repository tags
//	@ID				listGitHubRepositoryTags
//	@Description	Returns a strictly projected, cursor-paginated public tag page.
//	@Tags			github
//	@Produce		json,application/cbor
//	@Param			X-Request-ID	header		string	false	"Optional request correlation value"	minlength(1)	maxlength(128)
//	@Param			owner			path		string	true	"GitHub owner"							minlength(1)	maxlength(39)
//	@Param			repo			path		string	true	"GitHub repository"						minlength(1)	maxlength(100)
//	@Param			limit			query		int		false	"Page size"								default(20)		minimum(1)	maximum(100)
//	@Param			cursor			query		string	false	"Opaque scoped cursor"					minlength(1)	maxlength(2048)
//	@Success		200				{object}	TagPage
//	@Failure		400				{object}	respond.ProblemDetails
//	@Failure		404				{object}	respond.ProblemDetails
//	@Failure		406				{object}	respond.ProblemDetails
//	@Failure		422				{object}	respond.ProblemDetails
//	@Failure		429				{object}	respond.ProblemDetails
//	@Failure		500				{object}	respond.ProblemDetails
//	@Failure		502				{object}	respond.ProblemDetails
//	@Failure		504				{object}	respond.ProblemDetails
//	@Header			200				{string}	Link	"Optional RFC 8288 pagination links"
//	@Router			/v1/github/repos/{owner}/{repo}/tags [get]
func listRepositoryTags(service githubsvc.Service) echo.HandlerFunc {
	return func(c *echo.Context) error {
		owner, repository, err := repositoryInput(c, true)
		if err != nil {
			return err
		}
		limit, cursor, err := pageInput(c, pagination.Scope{
			Operation:  listTagsOperation,
			Owner:      owner,
			Repository: repository,
		})
		if err != nil {
			return err
		}
		result, err := service.ListRepositoryTags(c.Request().Context(), owner, repository, limit, cursor)
		if err != nil {
			return mapServiceError(c, "list repository tags", err)
		}
		setPageLink(c, "/v1/github/repos/"+owner+"/"+repository+"/tags", limit, result.NextCursor, result.PrevCursor)
		return respond.Negotiate(c, http.StatusOK, tagPageModel(result.Entries))
	}
}

func pointInput(c *echo.Context, repository bool) (string, error) {
	if err := request.RejectUnknownOrRepeatedQuery(c); err != nil {
		return "", err
	}
	return pathInput(c, repository)
}

func pathInput(c *echo.Context, repository bool) (string, error) {
	value := c.Param("owner")
	if repository {
		value = c.Param("repo")
	}
	if repository && !validRepository(value) || !repository && !validOwner(value) {
		return "", respond.ValidationFailed(respond.ErrorDetail{Detail: "Path parameter is invalid"})
	}
	return value, nil
}

func repositoryInput(c *echo.Context, paginated bool) (string, string, error) {
	if !paginated {
		if err := request.RejectUnknownOrRepeatedQuery(c); err != nil {
			return "", "", err
		}
	}
	owner := c.Param("owner")
	repository := c.Param("repo")
	if !validOwner(owner) || !validRepository(repository) {
		return "", "", respond.ValidationFailed(respond.ErrorDetail{Detail: "Path parameter is invalid"})
	}
	return owner, repository, nil
}

func pageInput(c *echo.Context, scope pagination.Scope) (int, *pagination.Cursor, error) {
	query, err := request.ParseQuery(c, "limit", "cursor")
	if err != nil {
		return 0, nil, err
	}
	limit, err := request.Limit(query)
	if err != nil {
		return 0, nil, err
	}
	scope.Limit = limit
	values, present := query["cursor"]
	if !present {
		return limit, nil, nil
	}
	cursor, err := pagination.DecodeCursor(values[0])
	if err != nil || !cursor.Matches(scope) {
		return 0, nil, respond.InvalidRequest()
	}
	return limit, &cursor, nil
}

func setPageLink(c *echo.Context, path string, limit int, nextCursor, previousCursor string) {
	link := pagination.BuildLinkHeader(path, url.Values{"limit": {strconv.Itoa(limit)}}, nextCursor, previousCursor)
	if link != "" {
		c.Response().Header().Set("Link", link)
	}
}

func mapServiceError(c *echo.Context, operation string, err error) error {
	ctx := c.Request().Context()
	var rateLimit *githubsvc.RateLimitError
	switch {
	case errors.Is(ctx.Err(), context.Canceled):
		return context.Canceled
	case errors.Is(ctx.Err(), context.DeadlineExceeded):
		return context.DeadlineExceeded
	case errors.Is(err, githubsvc.ErrInvalidCursor):
		return respond.InvalidRequest()
	case errors.Is(err, githubsvc.ErrNotFound):
		return respond.GitHubNotFound()
	case errors.As(err, &rateLimit):
		c.Response().Header().Set("Retry-After", rateLimit.RetryAfter)
		if rateLimit.Reset != "" {
			c.Response().Header().Set(rateLimitResetHeader, rateLimit.Reset)
		}
		return respond.GitHubRateLimit()
	case errors.Is(err, githubsvc.ErrTimeout):
		return respond.GitHubTimeout()
	case errors.Is(err, githubsvc.ErrUpstream):
		obs.Logger(ctx).Warn("GitHub operation failed", zap.String("operation", operation))
		return respond.GitHubUpstream()
	default:
		obs.Logger(ctx).Error("unexpected GitHub service failure", zap.String("operation", operation))
		return respond.InternalError()
	}
}

func validOwner(value string) bool {
	if len(value) < 1 || len(value) > 39 || !asciiAlphaNumeric(value[0]) {
		return false
	}
	if len(value) == 1 {
		return true
	}
	if !asciiAlphaNumeric(value[len(value)-1]) {
		return false
	}
	for index := 1; index < len(value)-1; index++ {
		if !asciiAlphaNumeric(value[index]) && value[index] != '_' && value[index] != '-' {
			return false
		}
	}
	return true
}

func validRepository(value string) bool {
	if len(value) < 1 || len(value) > 100 ||
		!strings.ContainsAny(value, "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789_-") {
		return false
	}
	for index := range len(value) {
		if !asciiAlphaNumeric(value[index]) && value[index] != '.' && value[index] != '_' && value[index] != '-' {
			return false
		}
	}
	return true
}

func asciiAlphaNumeric(value byte) bool {
	return value >= 'A' && value <= 'Z' || value >= 'a' && value <= 'z' || value >= '0' && value <= '9'
}
