package github

import (
	"github.com/janisto/echo-playground/internal/platform/timeutil"
	githubsvc "github.com/janisto/echo-playground/internal/service/github"
)

type Owner struct {
	ID          uint64        `json:"id"          cbor:"id"`
	Login       string        `json:"login"       cbor:"login"`
	Type        string        `json:"type"        cbor:"type"`
	Name        *string       `json:"name"        cbor:"name"`
	AvatarURL   string        `json:"avatarUrl"   cbor:"avatarUrl"`
	HTMLURL     string        `json:"htmlUrl"     cbor:"htmlUrl"`
	Company     *string       `json:"company"     cbor:"company"`
	Blog        *string       `json:"blog"        cbor:"blog"`
	Location    *string       `json:"location"    cbor:"location"`
	Bio         *string       `json:"bio"         cbor:"bio"`
	PublicRepos uint64        `json:"publicRepos" cbor:"publicRepos"`
	Followers   uint64        `json:"followers"   cbor:"followers"`
	Following   uint64        `json:"following"   cbor:"following"`
	CreatedAt   timeutil.Time `json:"createdAt"   cbor:"createdAt"`
	UpdatedAt   timeutil.Time `json:"updatedAt"   cbor:"updatedAt"`
}

type RepositorySummary struct {
	ID          uint64  `json:"id"          cbor:"id"`
	Name        string  `json:"name"        cbor:"name"`
	FullName    string  `json:"fullName"    cbor:"fullName"`
	Description *string `json:"description" cbor:"description"`
	HTMLURL     string  `json:"htmlUrl"     cbor:"htmlUrl"`
	Fork        bool    `json:"fork"        cbor:"fork"`
}

type Repository struct {
	RepositorySummary
	Language        *string        `json:"language"        cbor:"language"`
	StargazersCount uint64         `json:"stargazersCount" cbor:"stargazersCount"`
	ForksCount      uint64         `json:"forksCount"      cbor:"forksCount"`
	OpenIssuesCount uint64         `json:"openIssuesCount" cbor:"openIssuesCount"`
	Archived        bool           `json:"archived"        cbor:"archived"`
	CreatedAt       timeutil.Time  `json:"createdAt"       cbor:"createdAt"`
	UpdatedAt       timeutil.Time  `json:"updatedAt"       cbor:"updatedAt"`
	PushedAt        *timeutil.Time `json:"pushedAt"        cbor:"pushedAt"`
	DefaultBranch   string         `json:"defaultBranch"   cbor:"defaultBranch"`
	License         *string        `json:"license"         cbor:"license"`
	Topics          []string       `json:"topics"          cbor:"topics"`
	Disabled        bool           `json:"disabled"        cbor:"disabled"`
}

type Activity struct {
	ID             uint64        `json:"id"             cbor:"id"`
	Actor          *string       `json:"actor"          cbor:"actor"`
	ActorAvatarURL *string       `json:"actorAvatarUrl" cbor:"actorAvatarUrl"`
	Ref            string        `json:"ref"            cbor:"ref"`
	Timestamp      timeutil.Time `json:"timestamp"      cbor:"timestamp"`
	ActivityType   string        `json:"activityType"   cbor:"activityType"`
}

type Language struct {
	Name  string `json:"name"  cbor:"name"`
	Bytes uint64 `json:"bytes" cbor:"bytes"`
}

type TagCommit struct {
	SHA string `json:"sha" cbor:"sha"`
}

type Tag struct {
	Name   string    `json:"name"   cbor:"name"`
	Commit TagCommit `json:"commit" cbor:"commit"`
}

type RepositoryPage struct {
	Repos []RepositorySummary `json:"repos" cbor:"repos"`
	Count int                 `json:"count" cbor:"count"`
}

type ActivityPage struct {
	Activities []Activity `json:"activities" cbor:"activities"`
	Count      int        `json:"count"      cbor:"count"`
}

type Languages struct {
	Languages []Language `json:"languages" cbor:"languages"`
}

type TagPage struct {
	Tags  []Tag `json:"tags"  cbor:"tags"`
	Count int   `json:"count" cbor:"count"`
}

func ownerModel(value githubsvc.Owner) Owner {
	return Owner{
		ID: value.ID, Login: value.Login, Type: value.Type, Name: value.Name,
		AvatarURL: value.AvatarURL, HTMLURL: value.HTMLURL, Company: value.Company,
		Blog: value.Blog, Location: value.Location, Bio: value.Bio,
		PublicRepos: value.PublicRepos, Followers: value.Followers, Following: value.Following,
		CreatedAt: timeutil.NewTime(value.CreatedAt), UpdatedAt: timeutil.NewTime(value.UpdatedAt),
	}
}

func repositorySummaryModel(value githubsvc.RepositorySummary) RepositorySummary {
	return RepositorySummary{
		ID: value.ID, Name: value.Name, FullName: value.FullName,
		Description: value.Description, HTMLURL: value.HTMLURL, Fork: value.Fork,
	}
}

func repositoryModel(value githubsvc.Repository) Repository {
	var pushedAt *timeutil.Time
	if value.PushedAt != nil {
		converted := timeutil.NewTime(*value.PushedAt)
		pushedAt = &converted
	}
	return Repository{
		RepositorySummary: repositorySummaryModel(value.RepositorySummary),
		Language:          value.Language, StargazersCount: value.StargazersCount,
		ForksCount: value.ForksCount, OpenIssuesCount: value.OpenIssuesCount,
		Archived: value.Archived, CreatedAt: timeutil.NewTime(value.CreatedAt),
		UpdatedAt: timeutil.NewTime(value.UpdatedAt), PushedAt: pushedAt,
		DefaultBranch: value.DefaultBranch, License: value.License,
		Topics: append([]string(nil), value.Topics...), Disabled: value.Disabled,
	}
}

func repositoryPageModel(values []githubsvc.RepositorySummary) RepositoryPage {
	entries := make([]RepositorySummary, len(values))
	for index, value := range values {
		entries[index] = repositorySummaryModel(value)
	}
	return RepositoryPage{Repos: entries, Count: len(entries)}
}

func activityPageModel(values []githubsvc.Activity) ActivityPage {
	entries := make([]Activity, len(values))
	for index, value := range values {
		entries[index] = Activity{
			ID: value.ID, Actor: value.Actor, ActorAvatarURL: value.ActorAvatarURL,
			Ref: value.Ref, Timestamp: timeutil.NewTime(value.Timestamp), ActivityType: value.ActivityType,
		}
	}
	return ActivityPage{Activities: entries, Count: len(entries)}
}

func languagesModel(values []githubsvc.Language) Languages {
	entries := make([]Language, len(values))
	for index, value := range values {
		entries[index] = Language{Name: value.Name, Bytes: value.Bytes}
	}
	return Languages{Languages: entries}
}

func tagPageModel(values []githubsvc.Tag) TagPage {
	entries := make([]Tag, len(values))
	for index, value := range values {
		entries[index] = Tag{Name: value.Name, Commit: TagCommit{SHA: value.SHA}}
	}
	return TagPage{Tags: entries, Count: len(entries)}
}
