# User-Defined Filters

In addition to building a build-upon-able API for users, there's the other half where the platform gives a large
amount of customization to judge feed entries before they become approved into the timeline.
This is done using `Filter`'s, defined by the user and coming in a few supported flavors.

## Database representation

```go
// enum of all the different "filter features"
type FilterType string

const (
	FilterTypeAllowList    FilterType = "allow_list"
	FilterTypeDisallowList FilterType = "disallow_list"
	FilterTypeWebhook      FilterType = "webhook" // Not initially support, but main, planned feature
)

// Facilitates a bit of polymorphism since each config has a different shape.
type Filter interface {
	Type() FilterType
}

// Under the hood, the root table is `user_filters`, maybe specific to the `mysql` package:
type userFilter struct {
	ID     string     `db:"id"`
	UserID string     `db:"user_id"`
	Type   FilterType `db:"type"`
}

// That leads to a batch query against the other tables to assemble filters for a user.

type DisallowListConfig struct {
	ID       string   // ID of the original user_filter, but looks flattened when type coerced.
	UserID   string   // UserID of the original user_filter, but looks flattened when type coerced.
	Keywords []string // Defined as multiple rows in the database.
}

func (DisallowListConfig) Type() FilterType { return FilterTypeDisallowList }

type AllowListConfig struct {
	ID       string   // ID of the original user_filter, but looks flattened when type coerced.
	UserID   string   // UserID of the original user_filter, but looks flattened when type coerced.
	Keywords []string // Defined as multiple rows in the database.
}

func (AllowListConfig) Type() FilterType { return FilterTypeAllowList }

type WebhookConfig struct {
	ID     string // ID of the original user_filter, but looks flattened when type coerced.
	UserID string // UserID of the original user_filter, but looks flattened when type coerced.
	Host   string `db:"host"`
}

func (WebhookConfig) Type() FilterType { return FilterTypeWebhook }

// For webhooks we'd expect there to be a large number of tables driving that feature, like logging what was attempted and when.
```

## Filter API

### External

#### GET /users/{userID}/filters

Authz: Checks that the userID matches the current session.

Returns all filters for the user that they have created.

Response shape:
```go
type FilterType string

type Filter struct {
	ID string `json:"id"`
	Type FilterType `json:"type"`
	AllowListConfig AllowListConfig `json:"allow_list_config,omitempty"`
	DisllowListConfig DisllowListConfig `json:"disallow_list_config,omitempty"`
}

type AllowListConfig struct {
	Keywords []string `json:"keywords"`
}

type UserFiltersResp struct {
	Filters []Filter
}
```

NOTE: prefer a lack of polymorphism in the API layer. Internal layer it's more fine.

#### DELETE /filters/{filterID}

Authz: Checks that the user id on the filter matches the session

Removes the filter and returns a 202.

### Internal

The structs in "Database Representation", other than the mysql internal ones, should go in `seymour` alongside the existing `Timeline` service:

```go
type TimelineService interface {
	// New stuff:
	
	// CreateFilter inserts a new filter into storage.
	// 
	// Impl details: type switches on the Filter interface and knows how to persist based on the type.
	CreateFilter(context.Context, userID string, filter Filter) (string, error)

	// UserFilters fetches all filters tied to a user_id.
	// 
	// Impl details: switches based on the `user_filters` rows' type and then batches out to other tables to assemble
	// the final [Filter]'s being returned.
	UserFilters(context.Context, userID string) ([]Filter, error)
}
```

## Filter Application

Out of scope for now: How it gets applied during judgement of a timeline
