package model

import "strings"

// Column categories name what kind of work a column holds. The set is fixed
// and owned by kan: anything that reads across columns or boards reads
// categories, never column names, so each board keeps naming its columns as
// it likes.
const (
	CategoryInbox   = "inbox"
	CategoryIdea    = "idea"
	CategoryOpen    = "open"
	CategoryActive  = "active"
	CategoryReview  = "review"
	CategoryDone    = "done"
	CategoryDropped = "dropped"
)

// ValidCategories lists every column category, in workflow order.
var ValidCategories = []string{
	CategoryInbox, CategoryIdea, CategoryOpen, CategoryActive, CategoryReview, CategoryDone, CategoryDropped,
}

// IsValidCategory returns true if the given value is a column category. The
// empty string is valid and means the column has no category.
func IsValidCategory(c string) bool {
	if c == "" {
		return true
	}
	for _, valid := range ValidCategories {
		if c == valid {
			return true
		}
	}
	return false
}

// ValidCategoriesList returns the categories as a comma-separated list for messages.
func ValidCategoriesList() string {
	return strings.Join(ValidCategories, ", ")
}
