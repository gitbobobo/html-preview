package model

type Item struct {
	ID                string  `json:"id"`
	Title             string  `json:"title"`
	Notes             string  `json:"notes"`
	Status            string  `json:"status"`
	SourceKind        string  `json:"source_kind"`
	OriginalFilename  string  `json:"original_filename"`
	SizeBytes         int64   `json:"size_bytes"`
	ExpiresAt         *string `json:"expires_at"`
	TrashedAt         *string `json:"trashed_at"`
	Favorite          bool    `json:"favorite"`
	FavoritedAt       *string `json:"favorited_at"`
	ScreenshotStatus  string  `json:"screenshot_status"`
	ScreenshotError   *string `json:"screenshot_error"`
	CreatedAt         string  `json:"created_at"`
	UpdatedAt         string  `json:"updated_at"`
	PublicPath        string  `json:"public_path"`
	Thumbs            Thumbs  `json:"thumbs"`
}

type Thumbs struct {
	Desktop *string `json:"desktop"`
	Mobile  *string `json:"mobile"`
}
