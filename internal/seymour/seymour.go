// Package seymour holds Seymour's domain models and the Service interfaces
// (FeedService, TimelineService, UserService) that internal/mysql implements
// and internal/api and internal/worker depend on.
package seymour

//go:generate go tool mockgen -destination=../mock/seymour.go -package=mock . FeedService,TimelineService,UserService
