package profile

import (
	"errors"
	"net/http"

	"github.com/labstack/echo/v5"

	"github.com/janisto/echo-playground/internal/platform/auth"
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

func handleCreateProfile(svc profilesvc.Service) echo.HandlerFunc {
	return func(c *echo.Context) error {
		var input CreateInput
		if err := c.Bind(&input); err != nil {
			return err
		}
		if err := c.Validate(&input); err != nil {
			return err
		}

		if !input.Terms {
			return respond.Error422("terms must be accepted")
		}

		user, err := auth.UserFromEchoContext(c)
		if err != nil {
			return respond.Error401("unauthorized")
		}

		profile, err := svc.Create(c.Request().Context(), user.UID, profilesvc.CreateParams{
			Firstname:   input.Firstname,
			Lastname:    input.Lastname,
			Email:       input.Email,
			PhoneNumber: input.PhoneNumber,
			Marketing:   input.Marketing,
			Terms:       input.Terms,
		})
		if err != nil {
			return mapServiceError(err)
		}

		c.Response().Header().Set("Location", "/v1/profile")
		return respond.Negotiate(c, http.StatusCreated, toHTTPProfile(profile))
	}
}

func handleGetProfile(svc profilesvc.Service) echo.HandlerFunc {
	return func(c *echo.Context) error {
		user, err := auth.UserFromEchoContext(c)
		if err != nil {
			return respond.Error401("unauthorized")
		}

		profile, err := svc.Get(c.Request().Context(), user.UID)
		if err != nil {
			return mapServiceError(err)
		}

		return respond.Negotiate(c, http.StatusOK, toHTTPProfile(profile))
	}
}

func handleUpdateProfile(svc profilesvc.Service) echo.HandlerFunc {
	return func(c *echo.Context) error {
		var input UpdateInput
		if err := c.Bind(&input); err != nil {
			return err
		}
		if err := c.Validate(&input); err != nil {
			return err
		}

		user, err := auth.UserFromEchoContext(c)
		if err != nil {
			return respond.Error401("unauthorized")
		}

		profile, err := svc.Update(c.Request().Context(), user.UID, profilesvc.UpdateParams{
			Firstname:   input.Firstname,
			Lastname:    input.Lastname,
			Email:       input.Email,
			PhoneNumber: input.PhoneNumber,
			Marketing:   input.Marketing,
		})
		if err != nil {
			return mapServiceError(err)
		}

		return respond.Negotiate(c, http.StatusOK, toHTTPProfile(profile))
	}
}

func handleDeleteProfile(svc profilesvc.Service) echo.HandlerFunc {
	return func(c *echo.Context) error {
		user, err := auth.UserFromEchoContext(c)
		if err != nil {
			return respond.Error401("unauthorized")
		}

		if err := svc.Delete(c.Request().Context(), user.UID); err != nil {
			return mapServiceError(err)
		}

		return c.NoContent(http.StatusNoContent)
	}
}

func mapServiceError(err error) error {
	switch {
	case errors.Is(err, profilesvc.ErrNotFound):
		return respond.Error404("profile not found")
	case errors.Is(err, profilesvc.ErrAlreadyExists):
		return respond.Error409("profile already exists")
	default:
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
		Terms:       p.Terms,
		CreatedAt:   timeutil.Time{Time: p.CreatedAt},
		UpdatedAt:   timeutil.Time{Time: p.UpdatedAt},
	}
}
