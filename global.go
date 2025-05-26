package zapm

var Version string = "v0.0.0"

var BuildDate string = "1970-01-01"

var Conf *ZapmConfig = NewDefaultZapmConfig()

// LogsDir 全局日志目录路径
var LogsDir string
