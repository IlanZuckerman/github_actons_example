package version

//GitHash set by ld flags at build time
var GitHash = ""

//Version set by git tag
var Version = ""

//BuildDate set by ld flags at build time
var BuildDate = ""

//AppName Application name
var AppName = ""

func GetVersion() string {
	return Version
}

func GetBuildDate() string {
	return BuildDate
}

func GetRepoVersion() string {
	return GitHash
}

func GetAppName() string {
	return AppName
}
