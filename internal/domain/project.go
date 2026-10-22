package domain

// ProjectInfo describes a project created by goforge generate: how it was
// generated, what it serves, and whether GoForge can still extend it.
type ProjectInfo struct {
	AppName   string           `json:"app_name"`
	Module    string           `json:"module"`
	GoVersion string           `json:"go_version"`
	Port      int              `json:"port"`
	Features  ManifestFeatures `json:"features"`
	// Database is the database engine, e.g. "postgres", or "" for in-memory storage.
	Database  string         `json:"database,omitempty"`
	Resources []ResourceInfo `json:"resources"`
	// Routes lists every route the server registers.
	Routes []Route `json:"routes"`
	// Env lists the environment variables the server reads.
	Env []string `json:"env"`
	// Markers lists the comments `goforge add` edits files at.
	Markers []MarkerInfo `json:"markers"`
	// Problems lists anything that stops a `goforge add` command from working.
	Problems []string `json:"problems"`
}

// ResourceInfo describes a resource added with `goforge add resource`.
type ResourceInfo struct {
	Name    string  `json:"name"`
	Plural  string  `json:"plural"`
	Package string  `json:"package"`
	Path    string  `json:"path"`
	Fields  []Field `json:"fields"`
}

// Route is an HTTP route the server registers.
type Route struct {
	Method string `json:"method"`
	Path   string `json:"path"`
	// Resource is the resource that serves the route; empty for built-in routes.
	Resource string `json:"resource,omitempty"`
}

// MarkerInfo is a comment that `goforge add` inserts code above.
type MarkerInfo struct {
	File    string `json:"file"`
	Marker  string `json:"marker"`
	Present bool   `json:"present"`
	// UsedBy is the command that edits the file at the marker.
	UsedBy string `json:"used_by"`
}

// ProjectInspector describes existing projects.
type ProjectInspector interface {
	Inspect(dir string) (ProjectInfo, error)
}
