// Copyright 2025 SirSeer, LLC
//
// Licensed under the Business Source License 1.1 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     https://mariadb.com/bsl11
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package testutil

import (
	"fmt"
	"time"
)

// PullRequestBuilder provides a fluent API for creating test PRs
type PullRequestBuilder struct {
	number       int
	title        string
	state        string
	body         string
	author       string
	createdAt    time.Time
	updatedAt    time.Time
	mergedAt     *time.Time
	closedAt     *time.Time
	additions    int
	deletions    int
	changedFiles int
	comments     int
	commits      int
	labels       []string
	assignees    []string
}

// NewPullRequestBuilder creates a new PR builder with defaults
func NewPullRequestBuilder(number int) *PullRequestBuilder {
	now := time.Now()
	return &PullRequestBuilder{
		number:       number,
		title:        fmt.Sprintf("PR %d", number),
		state:        "OPEN",
		body:         fmt.Sprintf("This is the body of PR %d", number),
		author:       fmt.Sprintf("user%d", number),
		createdAt:    now.AddDate(0, 0, -number),
		updatedAt:    now.AddDate(0, 0, -number).Add(time.Hour),
		additions:    10,
		deletions:    5,
		changedFiles: 2,
		comments:     0,
		commits:      1,
	}
}

// WithTitle sets the PR title
func (b *PullRequestBuilder) WithTitle(title string) *PullRequestBuilder {
	b.title = title
	return b
}

// WithState sets the PR state (OPEN, CLOSED, MERGED)
func (b *PullRequestBuilder) WithState(state string) *PullRequestBuilder {
	b.state = state
	return b
}

// WithBody sets the PR body/description
func (b *PullRequestBuilder) WithBody(body string) *PullRequestBuilder {
	b.body = body
	return b
}

// WithAuthor sets the PR author
func (b *PullRequestBuilder) WithAuthor(author string) *PullRequestBuilder {
	b.author = author
	return b
}

// WithCreatedAt sets when the PR was created
func (b *PullRequestBuilder) WithCreatedAt(t time.Time) *PullRequestBuilder {
	b.createdAt = t
	return b
}

// WithMergedAt marks the PR as merged at the given time
func (b *PullRequestBuilder) WithMergedAt(t time.Time) *PullRequestBuilder {
	b.mergedAt = &t
	b.state = "MERGED"
	if b.closedAt == nil {
		b.closedAt = &t
	}
	return b
}

// WithClosedAt marks the PR as closed at the given time
func (b *PullRequestBuilder) WithClosedAt(t time.Time) *PullRequestBuilder {
	b.closedAt = &t
	if b.state == "OPEN" {
		b.state = "CLOSED"
	}
	return b
}

// WithChanges sets the additions/deletions/files
func (b *PullRequestBuilder) WithChanges(additions, deletions, files int) *PullRequestBuilder {
	b.additions = additions
	b.deletions = deletions
	b.changedFiles = files
	return b
}

// WithComments sets the comment count
func (b *PullRequestBuilder) WithComments(count int) *PullRequestBuilder {
	b.comments = count
	return b
}

// WithCommits sets the commit count
func (b *PullRequestBuilder) WithCommits(count int) *PullRequestBuilder {
	b.commits = count
	return b
}

// WithLabels adds labels to the PR
func (b *PullRequestBuilder) WithLabels(labels ...string) *PullRequestBuilder {
	b.labels = labels
	return b
}

// WithAssignees adds assignees to the PR
func (b *PullRequestBuilder) WithAssignees(assignees ...string) *PullRequestBuilder {
	b.assignees = assignees
	return b
}

// Build creates the PR data structure
func (b *PullRequestBuilder) Build() map[string]interface{} {
	labels := make([]map[string]interface{}, len(b.labels))
	for i, label := range b.labels {
		labels[i] = map[string]interface{}{
			"name": label,
		}
	}

	assignees := make([]map[string]interface{}, len(b.assignees))
	for i, assignee := range b.assignees {
		assignees[i] = map[string]interface{}{
			"login": assignee,
		}
	}

	pr := map[string]interface{}{
		"number":    b.number,
		"title":     b.title,
		"state":     b.state,
		"body":      b.body,
		"url":       fmt.Sprintf("https://github.com/test/repo/pull/%d", b.number),
		"createdAt": b.createdAt.Format(time.RFC3339),
		"updatedAt": b.updatedAt.Format(time.RFC3339),
		"author": map[string]interface{}{
			"login": b.author,
		},
		"baseRef": map[string]interface{}{
			"name": "main",
			"target": map[string]interface{}{
				"oid": fmt.Sprintf("base%d", b.number),
			},
		},
		"headRef": map[string]interface{}{
			"name": fmt.Sprintf("feature-%d", b.number),
			"target": map[string]interface{}{
				"oid": fmt.Sprintf("head%d", b.number),
			},
		},
		"additions":          b.additions,
		"deletions":          b.deletions,
		"changedFiles":       b.changedFiles,
		"comments":           b.comments,
		"reviewComments":     b.comments / 2, // Some comments are review comments
		"totalCommentsCount": b.comments,
		"commits": map[string]interface{}{
			"totalCount": b.commits,
		},
		"assignees": map[string]interface{}{
			"nodes": assignees,
		},
		"labels": map[string]interface{}{
			"nodes": labels,
		},
		"reviews": map[string]interface{}{
			"nodes": []interface{}{},
		},
		"reviewRequests": map[string]interface{}{
			"nodes": []interface{}{},
		},
		"merged": b.mergedAt != nil,
	}

	if b.mergedAt != nil {
		pr["mergedAt"] = b.mergedAt.Format(time.RFC3339)
	} else {
		pr["mergedAt"] = nil
	}

	if b.closedAt != nil {
		pr["closedAt"] = b.closedAt.Format(time.RFC3339)
	} else {
		pr["closedAt"] = nil
	}

	return pr
}

// GraphQLResponseBuilder builds GraphQL responses
type GraphQLResponseBuilder struct {
	prs         []map[string]interface{}
	hasNextPage bool
	endCursor   string
	errors      []map[string]interface{}
}

// NewGraphQLResponseBuilder creates a new response builder
func NewGraphQLResponseBuilder() *GraphQLResponseBuilder {
	return &GraphQLResponseBuilder{
		prs: []map[string]interface{}{},
	}
}

// WithPullRequests adds PRs to the response
func (b *GraphQLResponseBuilder) WithPullRequests(prs ...map[string]interface{}) *GraphQLResponseBuilder {
	b.prs = append(b.prs, prs...)
	return b
}

// WithPagination sets pagination info
func (b *GraphQLResponseBuilder) WithPagination(hasNext bool, cursor string) *GraphQLResponseBuilder {
	b.hasNextPage = hasNext
	b.endCursor = cursor
	return b
}

// WithError adds an error to the response
func (b *GraphQLResponseBuilder) WithError(message string) *GraphQLResponseBuilder {
	b.errors = append(b.errors, map[string]interface{}{
		"message": message,
	})
	return b
}

// Build creates the GraphQL response
func (b *GraphQLResponseBuilder) Build() map[string]interface{} {
	if len(b.errors) > 0 {
		return map[string]interface{}{
			"errors": b.errors,
		}
	}

	var cursor *string
	if b.endCursor != "" {
		cursor = &b.endCursor
	}

	return map[string]interface{}{
		"data": map[string]interface{}{
			"repository": map[string]interface{}{
				"pullRequests": map[string]interface{}{
					"nodes": b.prs,
					"pageInfo": map[string]interface{}{
						"hasNextPage": b.hasNextPage,
						"endCursor":   cursor,
					},
				},
			},
		},
	}
}
