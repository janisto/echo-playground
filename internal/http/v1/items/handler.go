package items

import (
	"net/http"
	"net/url"
	"slices"

	"github.com/labstack/echo/v5"

	"github.com/janisto/echo-playground/internal/platform/pagination"
	"github.com/janisto/echo-playground/internal/platform/respond"
)

const cursorType = "item"

// Register wires item routes into the provided group.
func Register(g *echo.Group) {
	g.GET("/items", listHandler)
}

// listHandler godoc
//
//	@Summary		List items
//	@Description	Returns a paginated list of items with optional category filtering
//	@Tags			items
//	@Accept			json
//	@Produce		json,application/cbor
//	@Param			cursor		query		string	false	"Pagination cursor"
//	@Param			limit		query		int		false	"Items per page"		minimum(1)	maximum(100)
//	@Param			category	query		string	false	"Filter by category"	Enums(electronics, tools, accessories, robotics, power, components)
//	@Success		200			{object}	ListData
//	@Failure		400			{object}	respond.ProblemDetails
//	@Failure		422			{object}	respond.ProblemDetails
//	@Header			200			{string}	Link	"RFC 8288 pagination links"
//	@Router			/items [get]
func listHandler(c *echo.Context) error {
	var input ListInput
	if err := c.Bind(&input); err != nil {
		return err
	}
	if err := c.Validate(&input); err != nil {
		return err
	}

	limit := input.Limit
	if limit == 0 {
		limit = pagination.DefaultLimit
	}

	cursor, err := pagination.DecodeCursor(input.Cursor)
	if err != nil {
		return respond.Error400("invalid cursor format")
	}

	if cursor.Type != "" && cursor.Type != cursorType {
		return respond.Error400("cursor type mismatch")
	}

	filtered := filterItems(mockItems, input.Category)

	if cursor.Value != "" && findItemIndex(filtered, cursor.Value) == -1 {
		return respond.Error400("cursor references unknown item")
	}

	query := url.Values{}
	if input.Category != "" {
		query.Set("category", input.Category)
	}

	result := pagination.Paginate(
		filtered,
		cursor,
		limit,
		cursorType,
		func(item Item) string { return item.ID },
		"/v1/items",
		query,
	)

	if result.LinkHeader != "" {
		c.Response().Header().Set("Link", result.LinkHeader)
	}
	return respond.Negotiate(c, http.StatusOK, ListData{
		Items: result.Items,
		Total: result.Total,
	})
}

func filterItems(items []Item, category string) []Item {
	if category == "" {
		return items
	}
	return slices.DeleteFunc(slices.Clone(items), func(item Item) bool {
		return item.Category != category
	})
}

func findItemIndex(items []Item, id string) int {
	return slices.IndexFunc(items, func(item Item) bool {
		return item.ID == id
	})
}
