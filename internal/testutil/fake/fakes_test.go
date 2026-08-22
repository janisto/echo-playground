package fake

import (
	"errors"
	"testing"

	githubsvc "github.com/janisto/echo-playground/internal/service/github"
)

func TestGitHubServiceFailsClosedWithoutProviderCalls(t *testing.T) {
	service := &GitHubService{}
	operations := []func() error{
		func() error { _, err := service.GetOwner(t.Context(), "owner"); return err },
		func() error { _, err := service.ListOwnerRepositories(t.Context(), "owner", 20, nil); return err },
		func() error { _, err := service.GetRepository(t.Context(), "owner", "repository"); return err },
		func() error {
			_, err := service.ListRepositoryActivity(t.Context(), "owner", "repository", 20, nil)
			return err
		},
		func() error {
			_, err := service.ListRepositoryLanguages(t.Context(), "owner", "repository")
			return err
		},
		func() error {
			_, err := service.ListRepositoryTags(t.Context(), "owner", "repository", 20, nil)
			return err
		},
	}
	for index, operation := range operations {
		if err := operation(); !errors.Is(err, githubsvc.ErrUpstream) {
			t.Fatalf("operation %d error = %v, want %v", index, err, githubsvc.ErrUpstream)
		}
	}
	if service.CallCount() != int32(len(operations)) { //nolint:gosec // bounded test slice length
		t.Fatalf("CallCount() = %d, want %d", service.CallCount(), len(operations))
	}
}
