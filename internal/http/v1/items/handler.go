package items

import (
	"net/http"
	"net/url"
	"slices"

	"github.com/labstack/echo/v5"

	"github.com/janisto/echo-playground/internal/platform/pagination"
	"github.com/janisto/echo-playground/internal/platform/request"
	"github.com/janisto/echo-playground/internal/platform/respond"
)

const operationID = "listItems"

// Register wires item routes into the provided group.
func Register(g *echo.Group) {
	g.GET("/items", listHandler, respond.SuccessNegotiation(false))
}

// listHandler returns the fixed item catalog with cursor pagination.
//
//	@Summary		List the fixed item catalog
//	@ID				listItems
//	@Description	Filters before pagination and rejects unknown or repeated query parameters.
//	@Tags			items
//	@Produce		json,application/cbor
//	@Param			X-Request-ID	header		string	false	"Optional request correlation value"	minlength(1)	maxlength(128)
//	@Param			limit			query		int		false	"Page size"								default(20)		minimum(1)	maximum(100)
//	@Param			cursor			query		string	false	"Opaque scoped cursor"					minlength(1)	maxlength(2048)
//	@Param			category		query		string	false	"Exact category filter"					Enums(electronics,tools,accessories,robotics,power,components)
//	@Success		200				{object}	ListData
//	@Failure		400				{object}	respond.ProblemDetails
//	@Failure		406				{object}	respond.ProblemDetails
//	@Failure		422				{object}	respond.ProblemDetails
//	@Failure		500				{object}	respond.ProblemDetails
//	@Header			200				{string}	Link	"Optional RFC 8288 pagination links"
//	@Router			/v1/items [get]
func listHandler(c *echo.Context) error {
	query, err := request.ParseQuery(c, "cursor", "limit", "category")
	if err != nil {
		return err
	}
	limit, err := request.Limit(query)
	if err != nil {
		return err
	}
	category := ""
	if categoryValues, present := query["category"]; present {
		category = categoryValues[0]
		if !validCategory(category) {
			return respond.ValidationFailed(respond.ErrorDetail{
				Detail: "category is not supported",
				Source: &respond.ErrorSource{Parameter: "category"},
			})
		}
	}
	scope := pagination.Scope{Operation: operationID, Filter: category, Limit: limit}
	var cursor *pagination.Cursor
	if cursorValues, present := query["cursor"]; present {
		decoded, decodeErr := pagination.DecodeCursor(cursorValues[0])
		if decodeErr != nil || !decoded.Matches(scope) {
			return respond.InvalidRequest()
		}
		cursor = &decoded
	}
	filtered := filterItems(catalog, category)

	linkQuery := url.Values{}
	if category != "" {
		linkQuery.Set("category", category)
	}
	result, err := pagination.Paginate(
		filtered,
		cursor,
		scope,
		func(item Item) string { return item.ID },
		c.Request().URL.Path,
		linkQuery,
	)
	if err != nil {
		return respond.InvalidRequest()
	}

	if result.LinkHeader != "" {
		c.Response().Header().Set("Link", result.LinkHeader)
	}
	return respond.Negotiate(c, http.StatusOK, ListData{
		Items: result.Items,
		Total: result.Total,
	})
}

func validCategory(value string) bool {
	switch value {
	case "electronics", "tools", "accessories", "robotics", "power", "components":
		return true
	default:
		return false
	}
}

func filterItems(items []Item, category string) []Item {
	if category == "" {
		return items
	}
	return slices.DeleteFunc(slices.Clone(items), func(item Item) bool {
		return item.Category != category
	})
}
