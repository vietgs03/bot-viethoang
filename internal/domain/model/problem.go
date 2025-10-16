package model

// Problem represents a LeetCode problem with metadata needed for notifications.
type Problem struct {
	ID         int
	Title      string
	Slug       string
	Difficulty string
	Link       string
	Content    string
	Topics     []string
}
