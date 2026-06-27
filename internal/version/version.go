package version

import "runtime"

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

type Info struct {
	Version string
	Commit  string
	Date    string
	Go      string
}

func Get() Info {
	return Info{
		Version: version,
		Commit:  commit,
		Date:    date,
		Go:      runtime.Version(),
	}
}
