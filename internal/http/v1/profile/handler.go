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

// handleCreateProfile godoc
//
//	@Summary		Create profile
//	@ID				createProfile
//	@Description	Creates a new user profile
//	@Tags			profile
//	@Produce		json,application/cbor
//	@Param			body	body		CreateInput	true	"Profile creation request body"
//	@Success		201		{object}	Profile
//	@Failure		400		{object}	respond.ProblemDetails
//	@Failure		401		{object}	respond.ProblemDetails
//	@Failure		406		{object}	respond.ProblemDetails
//	@Failure		409		{object}	respond.ProblemDetails
//	@Failure		413		{object}	respond.ProblemDetails
//	@Failure		415		{object}	respond.ProblemDetails
//	@Failure		422		{object}	respond.ProblemDetails
//	@Failure		500		{object}	respond.ProblemDetails
//	@Failure		503		{object}	respond.ProblemDetails
//	@Header			201		{string}	Location	"URI of the created profile"
//	@Security		BearerAuth
//	@Router			/profile [post]
func handleCreateProfile(svc profilesvc.Service) echo.HandlerFunc {
	return func(c *echo.Context) error {
		var input CreateInput
		if err := request.DecodeJSON(c, &input); err != nil {
			return err
		}
		if err := c.Validate(&input); err != nil {
			return err
		}

		user, err := auth.UserFromEchoContext(c)
		if err != nil {
			return respond.Error401("unauthorized")
		}

		ctx := c.Request().Context()
		profile, err := svc.Create(ctx, user.UID, profilesvc.CreateParams{
			Firstname:   input.Firstname,
			Lastname:    input.Lastname,
			Email:       input.Email,
			PhoneNumber: input.PhoneNumber,
			Marketing:   input.Marketing,
		})
		if err != nil {
			return mapServiceError(ctx, "create profile", err)
		}

		c.Response().Header().Set("Location", c.Request().URL.Path)
		return respond.Negotiate(c, http.StatusCreated, toHTTPProfile(profile))
	}
}

// handleGetProfile godoc
//
//	@Summary		Get profile
//	@ID				getProfile
//	@Description	Returns the authenticated user's profile
//	@Tags			profile
//	@Produce		json,application/cbor
//	@Success		200	{object}	Profile
//	@Failure		401	{object}	respond.ProblemDetails
//	@Failure		404	{object}	respond.ProblemDetails
//	@Failure		406	{object}	respond.ProblemDetails
//	@Failure		500	{object}	respond.ProblemDetails
//	@Failure		503	{object}	respond.ProblemDetails
//	@Security		BearerAuth
//	@Router			/profile [get]
func handleGetProfile(svc profilesvc.Service) echo.HandlerFunc {
	return func(c *echo.Context) error {
		user, err := auth.UserFromEchoContext(c)
		if err != nil {
			return respond.Error401("unauthorized")
		}

		ctx := c.Request().Context()
		profile, err := svc.Get(ctx, user.UID)
		if err != nil {
			return mapServiceError(ctx, "get profile", err)
		}

		return respond.Negotiate(c, http.StatusOK, toHTTPProfile(profile))
	}
}

// handleUpdateProfile godoc
//
//	@Summary		Update profile
//	@ID				updateProfile
//	@Description	Partially updates the authenticated user's profile
//	@Tags			profile
//	@Produce		json,application/cbor
//	@Param			body	body		UpdateInput	true	"Profile update request body"
//	@Success		200		{object}	Profile
//	@Failure		400		{object}	respond.ProblemDetails
//	@Failure		401		{object}	respond.ProblemDetails
//	@Failure		406		{object}	respond.ProblemDetails
//	@Failure		404		{object}	respond.ProblemDetails
//	@Failure		413		{object}	respond.ProblemDetails
//	@Failure		415		{object}	respond.ProblemDetails
//	@Failure		422		{object}	respond.ProblemDetails
//	@Failure		500		{object}	respond.ProblemDetails
//	@Failure		503		{object}	respond.ProblemDetails
//	@Security		BearerAuth
//	@Router			/profile [patch]
func handleUpdateProfile(svc profilesvc.Service) echo.HandlerFunc {
	return func(c *echo.Context) error {
		var input UpdateInput
		if err := request.DecodeJSON(c, &input); err != nil {
			return err
		}
		if err := c.Validate(&input); err != nil {
			return err
		}
		if input.Empty() {
			return respond.Error422("at least one field must be provided")
		}

		user, err := auth.UserFromEchoContext(c)
		if err != nil {
			return respond.Error401("unauthorized")
		}

		ctx := c.Request().Context()
		profile, err := svc.Update(ctx, user.UID, profilesvc.UpdateParams{
			Firstname:   input.Firstname,
			Lastname:    input.Lastname,
			Email:       input.Email,
			PhoneNumber: input.PhoneNumber,
			Marketing:   input.Marketing,
		})
		if err != nil {
			return mapServiceError(ctx, "update profile", err)
		}

		return respond.Negotiate(c, http.StatusOK, toHTTPProfile(profile))
	}
}

// handleDeleteProfile godoc
//
//	@Summary		Delete profile
//	@ID				deleteProfile
//	@Description	Deletes the authenticated user's profile
//	@Tags			profile
//	@Produce		json,application/cbor
//	@Success		204
//	@Failure		401	{object}	respond.ProblemDetails
//	@Failure		404	{object}	respond.ProblemDetails
//	@Failure		406	{object}	respond.ProblemDetails
//	@Failure		500	{object}	respond.ProblemDetails
//	@Failure		503	{object}	respond.ProblemDetails
//	@Security		BearerAuth
//	@Router			/profile [delete]
func handleDeleteProfile(svc profilesvc.Service) echo.HandlerFunc {
	return func(c *echo.Context) error {
		user, err := auth.UserFromEchoContext(c)
		if err != nil {
			return respond.Error401("unauthorized")
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
	case errors.Is(ctx.Err(), context.DeadlineExceeded):
		return respond.Error503("request deadline exceeded")
	case errors.Is(ctx.Err(), context.Canceled):
		return context.Canceled
	case errors.Is(err, profilesvc.ErrNotFound):
		return respond.Error404(profilesvc.ErrNotFound.Error())
	case errors.Is(err, profilesvc.ErrAlreadyExists):
		return respond.Error409(profilesvc.ErrAlreadyExists.Error())
	case errors.Is(err, profilesvc.ErrUnavailable):
		obs.Logger(ctx).Warn("profile dependency unavailable", zap.String("operation", operation))
		return respond.Error503("profile service temporarily unavailable")
	default:
		obs.Logger(ctx).Error("unexpected service error", zap.String("operation", operation), zap.Error(err))
		return respond.Error500("internal error")
	}
}

func toHTTPProfile(p *profilesvc.Profile) Profile {
	return Profile{
		ID:          p.ID,
		Firstname:   p.Firstname,
		Lastname:    p.Lastname,
		Email:       p.Email,
		PhoneNumber: p.PhoneNumber,
		Marketing:   p.Marketing,
		CreatedAt:   timeutil.Time{Time: p.CreatedAt},
		UpdatedAt:   timeutil.Time{Time: p.UpdatedAt},
	}
}
