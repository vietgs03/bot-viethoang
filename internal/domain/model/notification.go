package model

// NotificationField represents a titled section within a notification payload.
type NotificationField struct {
	Name   string
	Value  string
	Inline bool
}

// Notification is a transport-agnostic message for downstream notifiers.
type Notification struct {
	Title       string
	Description string
	Fields      []NotificationField
}
