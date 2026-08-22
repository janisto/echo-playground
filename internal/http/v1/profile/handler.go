package profile

import (
	"context"
	"errors"
	"net/http"

	"github.com/janisto/echo-observability/v2"
	"github.com/labstack/echo/v5"
	"go.uber.org/zap"

	"github.com/janisto/echo-playground/internal/platform/auth"
	"github.com/janisto/echo-playground/internal/platform/request"
	"github.com/janisto/echo-playground/internal/platform/respond"
	"github.com/janisto/echo-playground/internal/platform/timeutil"
	profilesvc "github.com/janisto/echo-playground/internal/service/profile"
)

// Register wires profile routes into the provided group.
// The group is expected to have auth middleware applied.
func Register(g *echo.Group, svc profilesvc.Service) {
	g.POST("/profile", handleCreateProfile(svc))
	g.GET("/profile", handleGetProfile(svc))
	g.PATCH("/profile", handleUpdateProfile(svc))
	g.DELETE("/profile", handleDeleteProfile(svc))
}

// handleCreateProfile creates the current principal's profile.
//
//	@Summary		Create the current principal profile
//	@ID				createProfile
//	@Description	Creates one profile owned by the verified Firebase principal. The query string is closed.
//	@Tags			profile
//	@Accept			json,application/cbor
//	@Produce		json,application/cbor
//	@Param			X-Request-ID	header		string		false	"Optional request correlation value"	minlength(1)	maxlength(128)
//	@Param			body			body		CreateInput	true	"Profile creation document"
//	@Success		201				{object}	Profile
//	@Failure		400				{object}	respond.ProblemDetails
//	@Failure		401				{object}	respond.ProblemDetails
//	@Failure		406				{object}	respond.ProblemDetails
//	@Failure		409				{object}	respond.ProblemDetails
//	@Failure		413				{object}	respond.ProblemDetails
//	@Failure		415				{object}	respond.ProblemDetails
//	@Failure		422				{object}	respond.ProblemDetails
//	@Failure		500				{object}	respond.ProblemDetails
//	@Failure		503				{object}	respond.ProblemDetails
//	@Header			201				{string}	Location			"Canonical profile location"
//	@Header			401				{string}	WWW-Authenticate	"Bearer challenge"
//	@Security		BearerAuth
//	@Router			/v1/profile [post]
func handleCreateProfile(svc profilesvc.Service) echo.HandlerFunc {
	return func(c *echo.Context) error {
		if err := request.RejectUnknownOrRepeatedQuery(c); err != nil {
			return err
		}
		var input CreateInput
		if err := request.Decode(c, &input); err != nil {
			return err
		}
		input.Normalize()
		if err := c.Validate(&input); err != nil {
			return err
		}

		user, err := auth.UserFromEchoContext(c)
		if err != nil {
			return respond.Unauthorized()
		}

		ctx := c.Request().Context()
		profile, err := svc.Create(ctx, user.UID, profilesvc.CreateParams{
			FirstName:      input.FirstName,
			LastName:       input.LastName,
			ContactEmail:   input.ContactEmail,
			PhoneNumber:    input.PhoneNumber,
			MarketingOptIn: input.MarketingOptIn.Value,
			TermsAccepted:  input.TermsAccepted,
		})
		if err != nil {
			return mapServiceError(ctx, "create profile", err)
		}

		c.Response().Header().Set("Location", c.Request().URL.Path)
		return respond.Negotiate(c, http.StatusCreated, toHTTPProfile(profile))
	}
}

// handleGetProfile gets the current principal's profile.
//
//	@Summary		Get the current principal profile
//	@ID				getProfile
//	@Description	Returns only the profile selected by the verified Firebase principal. The query string is closed.
//	@Tags			profile
//	@Produce		json,application/cbor
//	@Param			X-Request-ID	header		string	false	"Optional request correlation value"	minlength(1)	maxlength(128)
//	@Success		200				{object}	Profile
//	@Failure		400				{object}	respond.ProblemDetails
//	@Failure		401				{object}	respond.ProblemDetails
//	@Failure		404				{object}	respond.ProblemDetails
//	@Failure		406				{object}	respond.ProblemDetails
//	@Failure		500				{object}	respond.ProblemDetails
//	@Failure		503				{object}	respond.ProblemDetails
//	@Header			401				{string}	WWW-Authenticate	"Bearer challenge"
//	@Security		BearerAuth
//	@Router			/v1/profile [get]
func handleGetProfile(svc profilesvc.Service) echo.HandlerFunc {
	return func(c *echo.Context) error {
		if err := request.RejectUnknownOrRepeatedQuery(c); err != nil {
			return err
		}
		user, err := auth.UserFromEchoContext(c)
		if err != nil {
			return respond.Unauthorized()
		}

		ctx := c.Request().Context()
		profile, err := svc.Get(ctx, user.UID)
		if err != nil {
			return mapServiceError(ctx, "get profile", err)
		}

		return respond.Negotiate(c, http.StatusOK, toHTTPProfile(profile))
	}
}

// handleUpdateProfile updates the current principal's profile.
//
//	@Summary		Update the current principal profile
//	@ID				updateProfile
//	@Description	Atomically applies a non-empty partial update owned by the verified principal. The query string is closed.
//	@Tags			profile
//	@Accept			json,application/cbor
//	@Produce		json,application/cbor
//	@Param			X-Request-ID	header		string		false	"Optional request correlation value"	minlength(1)	maxlength(128)
//	@Param			body			body		UpdateInput	true	"Profile update document"
//	@Success		200				{object}	Profile
//	@Failure		400				{object}	respond.ProblemDetails
//	@Failure		401				{object}	respond.ProblemDetails
//	@Failure		404				{object}	respond.ProblemDetails
//	@Failure		406				{object}	respond.ProblemDetails
//	@Failure		413				{object}	respond.ProblemDetails
//	@Failure		415				{object}	respond.ProblemDetails
//	@Failure		422				{object}	respond.ProblemDetails
//	@Failure		500				{object}	respond.ProblemDetails
//	@Failure		503				{object}	respond.ProblemDetails
//	@Header			401				{string}	WWW-Authenticate	"Bearer challenge"
//	@Security		BearerAuth
//	@Router			/v1/profile [patch]
func handleUpdateProfile(svc profilesvc.Service) echo.HandlerFunc {
	return func(c *echo.Context) error {
		if err := request.RejectUnknownOrRepeatedQuery(c); err != nil {
			return err
		}
		var input UpdateInput
		if err := request.Decode(c, &input); err != nil {
			return err
		}
		input.Normalize()
		validationTarget := input.ValidationTarget()
		if err := c.Validate(&validationTarget); err != nil {
			return err
		}
		if input.Empty() {
			return respond.ValidationFailed()
		}

		user, err := auth.UserFromEchoContext(c)
		if err != nil {
			return respond.Unauthorized()
		}

		ctx := c.Request().Context()
		profile, err := svc.Update(ctx, user.UID, profilesvc.UpdateParams{
			FirstName:      input.FirstName.Pointer(),
			LastName:       input.LastName.Pointer(),
			ContactEmail:   input.ContactEmail.Pointer(),
			PhoneNumber:    input.PhoneNumber.Pointer(),
			MarketingOptIn: input.MarketingOptIn.Pointer(),
		})
		if err != nil {
			return mapServiceError(ctx, "update profile", err)
		}

		return respond.Negotiate(c, http.StatusOK, toHTTPProfile(profile))
	}
}

// handleDeleteProfile deletes the current principal's profile.
//
//	@Summary		Delete the current principal profile
//	@ID				deleteProfile
//	@Description	Atomically deletes only the profile owned by the verified principal. The query string is closed.
//	@Tags			profile
//	@Param			X-Request-ID	header	string	false	"Optional request correlation value"	minlength(1)	maxlength(128)
//	@Success		204
//	@Failure		400	{object}	respond.ProblemDetails
//	@Failure		401	{object}	respond.ProblemDetails
//	@Failure		404	{object}	respond.ProblemDetails
//	@Failure		500	{object}	respond.ProblemDetails
//	@Failure		503	{object}	respond.ProblemDetails
//	@Header			401	{string}	WWW-Authenticate	"Bearer challenge"
//	@Security		BearerAuth
//	@Router			/v1/profile [delete]
func handleDeleteProfile(svc profilesvc.Service) echo.HandlerFunc {
	return func(c *echo.Context) error {
		if err := request.RejectUnknownOrRepeatedQuery(c); err != nil {
			return err
		}
		user, err := auth.UserFromEchoContext(c)
		if err != nil {
			return respond.Unauthorized()
		}

		ctx := c.Request().Context()
		if err := svc.Delete(ctx, user.UID); err != nil {
			return mapServiceError(ctx, "delete profile", err)
		}

		return c.NoContent(http.StatusNoContent)
	}
}

func mapServiceError(ctx context.Context, operation string, err error) error {
	switch {
	case errors.Is(err, profilesvc.ErrNotFound):
		return respond.ProfileNotFound()
	case errors.Is(err, profilesvc.ErrAlreadyExists):
		return respond.ProfileExists()
	case errors.Is(ctx.Err(), context.Canceled):
		return context.Canceled
	case errors.Is(ctx.Err(), context.DeadlineExceeded):
		return respond.DependencyUnavailable()
	case errors.Is(err, profilesvc.ErrUnavailable):
		obs.Logger(ctx).Warn("profile dependency unavailable", zap.String("operation", operation))
		return respond.DependencyUnavailable()
	default:
		obs.Logger(ctx).Error("unexpected service error", zap.String("operation", operation))
		return respond.InternalError()
	}
}

func toHTTPProfile(p *profilesvc.Profile) Profile {
	return Profile{
		ID:             p.ID,
		FirstName:      p.FirstName,
		LastName:       p.LastName,
		ContactEmail:   p.ContactEmail,
		PhoneNumber:    p.PhoneNumber,
		MarketingOptIn: p.MarketingOptIn,
		TermsAccepted:  p.TermsAccepted,
		CreatedAt:      timeutil.Time{Time: p.CreatedAt},
		UpdatedAt:      timeutil.Time{Time: p.UpdatedAt},
	}
}
