package github

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestRepositoryDetailProjectionHandlesNullsAndScalarOrderedTopics(t *testing.T) {
	nullable, err := projectRepository([]byte(repositoryDetailFixture("null", "null", "")))
	if err != nil || nullable.PushedAt != nil || nullable.License != nil || len(nullable.Topics) != 0 {
		t.Fatalf("nullable repository = %#v, %v", nullable, err)
	}

	present, err := projectRepository([]byte(repositoryDetailFixture(
		`"2026-07-30T12:02:00.000Z"`, `{"spdx_id":"Apache-2.0"}`, `,"topics":["𐀀","\uE000"]`,
	)))
	if err != nil || present.PushedAt == nil || present.PushedAt.Format(time.RFC3339Nano) != "2026-07-30T12:02:00Z" ||
		present.License == nil || *present.License != "Apache-2.0" || len(present.Topics) != 2 ||
		present.Topics[0] != "\uE000" || present.Topics[1] != "\U00010000" {
		t.Fatalf("present repository = %#v, %v", present, err)
	}

	for _, test := range []struct {
		name string
		body string
	}{
		{name: "duplicate topics", body: repositoryDetailFixture("null", "null", `,"topics":["go","go"]`)},
		{name: "private", body: strings.Replace(repositoryDetailFixture("null", "null", ""), `"private":false`, `"private":true`, 1)},
		{name: "nonpublic visibility", body: strings.Replace(repositoryDetailFixture("null", "null", ""), `"visibility":"public"`, `"visibility":"private"`, 1)},
		{name: "non-millisecond timestamp", body: repositoryDetailFixture(`"2026-07-30T12:02:00.0001Z"`, "null", "")},
		{name: "unsafe integer", body: strings.Replace(repositoryDetailFixture("null", "null", ""), `"stargazers_count":2`, `"stargazers_count":9007199254740992`, 1)},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, err := projectRepository([]byte(test.body)); !errors.Is(err, ErrUpstream) {
				t.Fatalf("projection error = %v", err)
			}
		})
	}
}

func TestCollectionProjectionsPreserveOrDefineExactOrder(t *testing.T) {
	repositories, err := projectRepositorySummaries(
		[]byte(`[`+repositorySummaryFixture("second")+`,`+repositorySummaryFixture("first")+`]`),
		2,
	)
	if err != nil || repositories[0].Name != "second" || repositories[1].Name != "first" {
		t.Fatalf("repository order = %#v, %v", repositories, err)
	}

	activitiesJSON := `[` + activityFixture(
		"2",
		`{"login":"octo","avatar_url":"https://example.test/octo"}`,
		"2026-07-30T12:02:00.000Z",
	) + `,` +
		activityFixture(
			"1",
			"null",
			"2026-07-30T12:01:00.000Z",
		) + `]`
	activities, err := projectActivities([]byte(activitiesJSON), 2)
	if err != nil || len(activities) != 2 || activities[0].Actor == nil || *activities[0].Actor != "octo" ||
		activities[0].ActorAvatarURL == nil || activities[1].Actor != nil || activities[1].ActorAvatarURL != nil || activities[0].ID != 2 {
		t.Fatalf("activities = %#v, %v", activities, err)
	}

	languages, err := projectLanguages([]byte(`{"𐀀":7,"\uE000":7,"Go":9}`))
	if err != nil || len(languages) != 3 || languages[0] != (Language{Name: "Go", Bytes: 9}) ||
		languages[1] != (Language{Name: "\uE000", Bytes: 7}) || languages[2] != (Language{Name: "\U00010000", Bytes: 7}) {
		t.Fatalf("languages = %#v, %v", languages, err)
	}
	emptyLanguages, err := projectLanguages([]byte(`{}`))
	if err != nil || emptyLanguages == nil || len(emptyLanguages) != 0 {
		t.Fatalf("empty languages = %#v, %v", emptyLanguages, err)
	}

	sha40, sha64 := strings.Repeat("a", 40), strings.Repeat("b", 64)
	tags, err := projectTags(
		fmt.Appendf(nil, `[{"name":"v1","commit":{"sha":%q}},{"name":"v2","commit":{"sha":%q}}]`, sha40, sha64),
		2,
	)
	if err != nil || len(tags) != 2 || tags[0].SHA != sha40 || tags[1].SHA != sha64 {
		t.Fatalf("tags = %#v, %v", tags, err)
	}
}

func TestEveryPaginatedProjectionRejectsRatherThanTruncatesOverLimit(t *testing.T) {
	tests := []struct {
		name    string
		project func() error
	}{
		{name: "repositories", project: func() error {
			_, err := projectRepositorySummaries(
				[]byte(`[`+repositorySummaryFixture("one")+`,`+repositorySummaryFixture("two")+`]`),
				1,
			)
			return err
		}},
		{name: "activities", project: func() error {
			body := `[` + activityFixture(
				"1",
				"null",
				"2026-07-30T12:01:00.000Z",
			) + `,` + activityFixture(
				"2",
				"null",
				"2026-07-30T12:02:00.000Z",
			) + `]`
			_, err := projectActivities([]byte(body), 1)
			return err
		}},
		{name: "tags", project: func() error {
			sha := strings.Repeat("a", 40)
			_, err := projectTags(
				fmt.Appendf(nil, `[{"name":"one","commit":{"sha":%q}},{"name":"two","commit":{"sha":%q}}]`, sha, sha),
				1,
			)
			return err
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := test.project(); !errors.Is(err, ErrUpstream) {
				t.Fatalf("projection error = %v", err)
			}
		})
	}
}

func TestProjectionRejectsInconsistentOrMalformedActorAndTag(t *testing.T) {
	for _, actor := range []string{
		`{"login":"octo"}`,
		`{"login":"octo","avatar_url":"not-a-url"}`,
		`"octo"`,
	} {
		if _, err := projectActivities(
			[]byte(`[`+activityFixture("1", actor, "2026-07-30T12:01:00.000Z")+`]`),
			1,
		); !errors.Is(
			err,
			ErrUpstream,
		) {
			t.Fatalf("actor %s error = %v", actor, err)
		}
	}
	for _, sha := range []string{strings.Repeat("A", 40), strings.Repeat("a", 39), strings.Repeat("g", 40)} {
		body := fmt.Sprintf(`[{"name":"v1","commit":{"sha":%q}}]`, sha)
		if _, err := projectTags([]byte(body), 1); !errors.Is(err, ErrUpstream) {
			t.Fatalf("sha %q error = %v", sha, err)
		}
	}
}

func TestProjectionRequiresCanonicalProviderURI(t *testing.T) {
	canonical := "https://example.test/a%20b?size=109&next=%2F"
	body := strings.Replace(ownerFixture(""), "https://example.test/avatar", canonical, 1)
	owner, err := projectOwner([]byte(body))
	if err != nil || owner.AvatarURL != canonical {
		t.Fatalf("canonical provider URI = %q, %v", owner.AvatarURL, err)
	}

	for _, value := range []string{
		"https://example.test/a b",
		"https://example.test/a?q=b c",
		"https://example.test/a?q=%",
		"https://example.test/a?q=%A",
		"https://example.test/a?q=%ZZ",
		"https://example.test/a?q=[AA",
		"https://example.test/a?q=[]",
		"https://example.test/café",
	} {
		t.Run(value, func(t *testing.T) {
			body := strings.Replace(ownerFixture(""), "https://example.test/avatar", value, 1)
			if _, err := projectOwner([]byte(body)); !errors.Is(err, ErrUpstream) {
				t.Fatalf("provider URI %q error = %v", value, err)
			}
		})
	}
}

func repositoryDetailFixture(pushedAt, license, topics string) string {
	return `{"id":1,"name":"repo","full_name":"acme/repo","description":null,"html_url":"https://example.test/acme/repo","fork":false,"private":false,"visibility":"public","language":null,"stargazers_count":2,"forks_count":3,"open_issues_count":4,"archived":false,"created_at":"2026-07-30T12:00:00.000Z","updated_at":"2026-07-30T12:01:00.000Z","pushed_at":` + pushedAt + `,"default_branch":"main","license":` + license + topics + `,"disabled":false}`
}

func activityFixture(id, actor, timestamp string) string {
	return `{"id":` + id + `,"actor":` + actor + `,"ref":"refs/heads/main","timestamp":"` + timestamp + `","activity_type":"push"}`
}
