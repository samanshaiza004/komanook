# Reader profiles reference Works

Reader identity lives in an optional profile separate from login data; public profile routes expose only explicitly public profile fields, selected Work references, and reading aggregates the reader enables. Favorites reference canonical Works, while the current hosted series remains edition-scoped and chapter progress stays edition-specific. The first showcase configuration is constrained and versioned in PostgreSQL, so profiles can grow without user HTML/CSS or an opaque, unvalidated layout blob.
